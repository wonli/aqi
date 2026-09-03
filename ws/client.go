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
	Endpoint       string    //入口地址
	OnceId         string    //临时ID，扫码登录等场景作为客户端唯一标识
	ClientId       string    //客户端ID
	Disconnecting  bool      //已被设置为断开状态（消息发送完之后断开连接）
	SyncMsg        bool      //是否接收消息
	LastMsgId      int       //最后一条消息ID
	RequiredValid  bool      //人机验证标识
	Validated      bool      //是否已验证
	ValidExpiry    time.Time //验证有效期
	ValidCacheData any       //验证相关缓存数据
	AuthCode       string    //用于校验JWT中的code，如果相等识别为同一个用户的网络地址变更
	ErrorCount     int       //错误次数

	Limiter      *rate.Limiter //限速器
	RequestQueue chan string   //处理队列

	HttpRequest *http.Request
	HttpWriter  http.ResponseWriter
	ctx         context.Context
	cancel      context.CancelFunc

	User              *User     //关联用户
	Scope             string    //登录jwt scope, 用于判断用户从哪里登录的
	AppId             string    //登录应用Id
	TenantId          uint      //租户ID
	Version           string    //客户端版本号
	Platform          string    //登录平台
	IsLogin           bool      //是否已登录
	LoginAction       string    //登录动作
	ForceDialogId     string    //打开聊天界面的会话ID
	IpAddress         string    //IP地址
	IpAddressPort     string    //IP地址和端口
	IpLocation        string    //通过IP转换获得的地理位置
	ConnectionTime    time.Time //连接时间
	LastRequestTime   time.Time //最后请求时间
	LastHeartbeatTime time.Time //最后心跳活动时间

	disconnectOnce sync.Once
	stateMu        sync.RWMutex
	mu             sync.RWMutex
	Keys           map[string]any

	// recent logs ring buffer (last 100 items)
	recentLogs  [100]string
	recentIdx   int
	recentCount int
}

// initContext creates a context whose lifetime follows the WebSocket
// connection instead of the HTTP request used to perform the upgrade. The
// HTTP handler returns immediately after the upgrade, which cancels the
// request context while the WebSocket remains active.
func (c *Client) initContext(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	c.ctx, c.cancel = context.WithCancel(context.WithoutCancel(parent))
}

// Context returns the context associated with the WebSocket connection.
func (c *Client) Context() context.Context {
	if c == nil || c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// setLoginState associates the client with a logged-in user.
func (c *Client) setLoginState(user *User, appId string) {
	c.stateMu.Lock()
	c.User = user
	c.AppId = appId
	c.IsLogin = true
	c.stateMu.Unlock()
}

// LoginState returns a consistent snapshot of the client's login identity.
func (c *Client) LoginState() (*User, string, bool) {
	if c == nil {
		return nil, "", false
	}
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.User, c.AppId, c.IsLogin
}

// IsLoggedIn reports whether the client is currently associated with a user.
func (c *Client) IsLoggedIn() bool {
	_, _, loggedIn := c.LoginState()
	return loggedIn
}

// SetLastHeartbeat updates the client's heartbeat timestamp and mirrors it to
// the associated user when present.
func (c *Client) SetLastHeartbeat(t time.Time) {
	c.stateMu.Lock()
	c.LastHeartbeatTime = t
	user := c.User
	c.stateMu.Unlock()

	if user != nil {
		user.SetLastHeartbeat(t)
	}
}

// TouchRequest records request activity and initializes the heartbeat time for
// newly active connections.
func (c *Client) TouchRequest(t time.Time) {
	c.stateMu.Lock()
	c.LastRequestTime = t
	if c.LastHeartbeatTime.IsZero() {
		c.LastHeartbeatTime = t
	}
	c.stateMu.Unlock()
}

// Disconnect terminates the client lifecycle exactly once and notifies Hub
// to clean up registries and user state.
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

		c.Log("xx", fmt.Sprintf("Close client -> %s", c.IpAddressPort))

		if c.Hub != nil {
			c.Hub.Disconnect <- c
		}
	})
}

// handleControlFrame handles an already-unmasked WebSocket control payload.
// It never writes to Conn directly; outbound control frames are queued for Writer.
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

// Reader reads WebSocket frames without using wsutil.ReadClientData because
// that helper may write Pong/Close frames from the read goroutine. Reader must
// remain read-only; Writer is the sole owner of outbound socket writes.
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
			c.Log("xx", "Error reading data", err.Error())
			return
		}

		payload, err := io.ReadAll(reader)
		if err != nil {
			if errors.Is(err, errPeerClose) {
				writerOwnsDisconnect = true
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

// Request 处理请求
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

// Write 发送
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

			//如果设置为断开状态
			//在消息发送完成后将断开与服务器的连接
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

// Log websocket日志
func (c *Client) Log(symbol string, msg ...string) {
	s := strings.Join(msg, ", ")
	user, appId, loggedIn := c.LoginState()
	if loggedIn && user != nil {
		s = fmt.Sprintf("%s %s [%s-%s] %s", c.IpAddressPort, symbol, user.Suid, appId, s)
	} else {
		s = fmt.Sprintf("%s %s %s", c.IpAddressPort, symbol, s)
	}

	if logger.SugarLog != nil {
		logger.SugarLog.Info(s)
	}

	c.mu.Lock()
	c.recentLogs[c.recentIdx] = fmt.Sprintf("%s %s", time.Now().Format(time.RFC3339), s)
	c.recentIdx = (c.recentIdx + 1) % 100
	if c.recentCount < 100 {
		c.recentCount++
	}
	c.mu.Unlock()
}

// sendControlMessage queues a control frame and waits for queue capacity.
// Control frames must not be silently dropped when the regular send queue is full.
func (c *Client) sendControlMessage(msg Message) error {
	select {
	case <-c.Context().Done():
		return c.Context().Err()
	case c.Send <- msg:
		return nil
	}
}

// SendMessage queues an outbound WebSocket frame without blocking.
func (c *Client) SendMessage(msg Message) {
	select {
	case <-c.Context().Done():
		return
	case c.Send <- msg:
	default:
	}
}

// SendMsg 把文本消息加入发送队列
func (c *Client) SendMsg(msg []byte) {
	c.SendMessage(Message{Op: ws.OpText, Data: msg})
}

// SendActionMsg 构造消息再发送
func (c *Client) SendActionMsg(a *Action) {
	c.SendMsg(a.Encode())
}

// Close 关闭客户端
// Deprecated: use Disconnect. Kept as an alias for compatibility.
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
