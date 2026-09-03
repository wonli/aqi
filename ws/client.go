package ws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"golang.org/x/time/rate"

	"github.com/wonli/aqi/logger"
)

var errPeerClose = errors.New("peer requested websocket close")

type Client struct {
	Hub            *Hubc
	Conn           net.Conn
	Send           chan Message
	Endpoint       string
	OnceId         string
	ClientId       string
	Disconnecting  bool
	SyncMsg        bool
	LastMsgId      int
	RequiredValid  bool
	Validated      bool
	ValidExpiry    time.Time
	ValidCacheData any
	AuthCode       string
	ErrorCount     int

	Limiter      *rate.Limiter
	RequestQueue chan string

	HttpRequest *http.Request
	HttpWriter  http.ResponseWriter
	ctx         context.Context
	cancel      context.CancelFunc

	User              *User
	Scope             string
	AppId             string
	TenantId          uint
	Version           string
	Platform          string
	IsLogin           bool
	LoginAction       string
	ForceDialogId     string
	IpAddress         string
	IpAddressPort     string
	IpLocation        string
	ConnectionTime    time.Time
	LastRequestTime   time.Time
	LastHeartbeatTime time.Time

	disconnectOnce sync.Once
	stateMu        sync.RWMutex
	mu             sync.RWMutex
	Keys           map[string]any

	recentLogs  [100]string
	recentIdx   int
	recentCount int
}

func (c *Client) initContext(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	c.ctx, c.cancel = context.WithCancel(context.WithoutCancel(parent))
}

func (c *Client) Context() context.Context {
	if c == nil || c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *Client) setLoginState(user *User, appId string) {
	c.stateMu.Lock()
	c.User = user
	c.AppId = appId
	c.IsLogin = true
	c.stateMu.Unlock()
}

func (c *Client) LoginState() (*User, string, bool) {
	if c == nil {
		return nil, "", false
	}
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.User, c.AppId, c.IsLogin
}

func (c *Client) IsLoggedIn() bool {
	_, _, loggedIn := c.LoginState()
	return loggedIn
}

func (c *Client) SetLastHeartbeat(t time.Time) {
	c.stateMu.Lock()
	c.LastHeartbeatTime = t
	user := c.User
	c.stateMu.Unlock()

	if user != nil {
		user.SetLastHeartbeat(t)
	}
}

func (c *Client) TouchRequest(t time.Time) {
	c.stateMu.Lock()
	c.LastRequestTime = t
	if c.LastHeartbeatTime.IsZero() {
		c.LastHeartbeatTime = t
	}
	c.stateMu.Unlock()
}

func (c *Client) Disconnect() {
	if c == nil {
		return
	}

	c.disconnectOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}

		if c.Conn != nil {
			_ = c.Conn.Close()
		}

		c.Log("--", "disconnect")

		if c.Hub != nil {
			c.Hub.Disconnect <- c
		}
	})
}

func (c *Client) handleControlFrame(op ws.OpCode, payload []byte) error {
	switch op {
	case ws.OpPing:
		return c.sendControlMessage(Message{Op: ws.OpPong, Data: payload})

	case ws.OpPong:
		c.SetLastHeartbeat(time.Now())
		return nil

	case ws.OpClose:
		response := payload
		if len(payload) > 0 {
			code, reason := ws.ParseCloseFrameData(payload)
			if err := ws.CheckCloseFrameData(code, reason); err != nil {
				response = ws.NewCloseFrameBody(ws.StatusProtocolError, err.Error())
			}
		}

		if err := c.sendControlMessage(Message{Op: ws.OpClose, Data: response}); err != nil {
			return err
		}
		return errPeerClose
	}

	return nil
}

func (c *Client) Reader() {
	writerOwnsDisconnect := false
	defer func() {
		if !writerOwnsDisconnect {
			c.Disconnect()
		}
	}()

	reader := wsutil.NewReader(c.Conn, ws.StateServerSide)
	reader.CheckUTF8 = true
	reader.OnIntermediate = func(hdr ws.Header, src io.Reader) error {
		payload, err := io.ReadAll(src)
		if err != nil {
			return err
		}
		return c.handleControlFrame(hdr.OpCode, payload)
	}

	for {
		hdr, err := reader.NextFrame()
		if err != nil {
			if errors.Is(err, errPeerClose) {
				writerOwnsDisconnect = true
				return
			}
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			c.Log("xx", "Error reading data", err.Error())
			return
		}

		payload, err := io.ReadAll(reader)
		if err != nil {
			if errors.Is(err, errPeerClose) {
				writerOwnsDisconnect = true
				return
			}
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			c.Log("xx", "Error reading data", err.Error())
			return
		}

		if hdr.OpCode.IsControl() {
			if err := c.handleControlFrame(hdr.OpCode, payload); err != nil {
				if errors.Is(err, errPeerClose) {
					writerOwnsDisconnect = true
					return
				}
				return
			}
			continue
		}

		switch hdr.OpCode {
		case ws.OpText:
			req := string(payload)
			c.Log("<-", req)
			select {
			case c.RequestQueue <- req:
			case <-c.Context().Done():
				return
			}

		case ws.OpBinary:
			c.Log("xx", "Unrecognized binary message")

		default:
			c.Log("xx", "Unrecognized action")
		}
	}
}

func (c *Client) Request() {
	for {
		select {
		case <-c.Context().Done():
			return

		case req, ok := <-c.RequestQueue:
			if !ok {
				return
			}

			if !c.Limiter.Allow() {
				c.Log("!!", "Too many requests, please retry later")
				c.SendActionMsg(&Action{
					Action: "sys.rateLimit",
					Code:   -1003,
					Msg:    "too many requests, please retry later",
				})
				continue
			}

			Dispatcher(c, req)
		}
	}
}

func (c *Client) writeMessage(msg Message) error {
	if err := wsutil.WriteServerMessage(c.Conn, msg.Op, msg.Data); err != nil {
		c.Log("xx", "Send msg error", err.Error())
		return err
	}

	if msg.Op == ws.OpText {
		c.Log("->", string(msg.Data))
	}

	return nil
}

func (c *Client) Write() {
	timer := time.NewTicker(5 * time.Second)
	defer func() {
		timer.Stop()
		c.Disconnect()
	}()

	for {
		select {
		case <-c.Context().Done():
			return

		case msg := <-c.Send:
			if err := c.writeMessage(msg); err != nil {
				return
			}

			if msg.Op == ws.OpClose {
				return
			}

			if c.Disconnecting {
				return
			}

		case <-timer.C:
			if err := c.writeMessage(Message{Op: ws.OpPing, Data: []byte("ping")}); err != nil {
				return
			}
		}
	}
}

// Log records the in-memory client trace synchronously, but sends persistent
// websocket ledger output through a bounded async queue so disk I/O cannot
// inflate request, response, connect, or disconnect latency.
func (c *Client) Log(symbol string, msg ...string) {
	s := strings.Join(msg, ", ")
	user, appId, loggedIn := c.LoginState()
	if loggedIn && user != nil {
		s = fmt.Sprintf("%s %s [%s-%s] %s", c.IpAddressPort, symbol, user.Suid, appId, s)
	} else {
		s = fmt.Sprintf("%s %s %s", c.IpAddressPort, symbol, s)
	}

	log := logger.FileLog
	enqueueWebsocketLog(log, s)

	now := time.Now().Format(time.RFC3339)
	c.mu.Lock()
	c.recentLogs[c.recentIdx] = fmt.Sprintf("%s %s", now, s)
	c.recentIdx = (c.recentIdx + 1) % 100
	if c.recentCount < 100 {
		c.recentCount++
	}
	c.mu.Unlock()
}

func (c *Client) sendControlMessage(msg Message) error {
	select {
	case <-c.Context().Done():
		return c.Context().Err()
	case c.Send <- msg:
		return nil
	}
}

func (c *Client) SendMessage(msg Message) {
	select {
	case <-c.Context().Done():
		return
	case c.Send <- msg:
	default:
	}
}

func (c *Client) SendMsg(msg []byte) {
	c.SendMessage(Message{Op: ws.OpText, Data: msg})
}

func (c *Client) SendActionMsg(a *Action) {
	c.SendMsg(a.Encode())
}

func (c *Client) Close() {
	c.Disconnect()
}

func (c *Client) GetRecentLogs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := c.recentCount
	if count == 0 {
		return nil
	}

	res := make([]string, count)
	oldest := (c.recentIdx - count + 100) % 100
	for i := 0; i < count; i++ {
		idx := (oldest + i) % 100
		res[i] = c.recentLogs[idx]
	}

	return res
}

func (c *Client) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}
