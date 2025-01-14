package ws

import (
	"fmt"
	"github.com/gobwas/ws"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gobwas/ws/wsutil"

	"github.com/wonli/aqi/logger"
)

type Client struct {
	Hub            *Hubc       `json:"-"`
	Conn           net.Conn    `json:"-"`
	Send           chan []byte `json:"-"`
	Endpoint       string      `json:"-"` //入口地址
	OnceId         string      `json:"-"` //临时ID，扫码登录等场景作为客户端唯一标识
	ClientId       string      `json:"-"` //客户端ID
	Disconnecting  bool        `json:"-"` //已被设置为断开状态（消息发送完之后断开连接）
	SyncMsg        bool        `json:"-"` //是否接收消息
	LastMsgId      int         `json:"-"` //最后一条消息ID
	RequiredValid  bool        `json:"-"` //人机验证标识
	Validated      bool        `json:"-"` //是否已验证
	ValidExpiry    time.Time   `json:"-"` //验证有效期
	ValidCacheData any         `json:"-"` //验证相关缓存数据
	AuthCode       string      `json:"-"` //用于校验JWT中的code，如果相等识别为同一个用户的网络地址变更
	ErrorCount     int         `json:"-"` //错误次数
	Closed         bool        `json:"-"` //是否已经关闭

	HttpRequest *http.Request       `json:"-"`
	HttpWriter  http.ResponseWriter `json:"-"`

	User              *User     `json:"user,omitempty"` //关联用户
	Scope             string    `json:"scope"`          //登录jwt scope, 用于判断用户从哪里登录的
	AppId             string    `json:"appId"`          //登录应用Id
	StoreId           uint      `json:"storeId"`        //店铺ID
	MerchantId        uint      `json:"merchantId"`     //商户ID
	TenantId          uint      `json:"tenantId"`       //租户ID
	Platform          string    `json:"platform"`       //登录平台
	GroupId           string    `json:"groupId"`        //用户分组Id
	IsLogin           bool      `json:"isLogin"`        //是否已登录
	LoginAction       string    `json:"loginAction"`    //登录动作
	ForceDialogId     string    `json:"forceDialogId"`  //打开聊天界面的会话ID
	IpAddress         string    `json:"ipAddress"`      //IP地址
	IpLocation        string    `json:"ipLocation"`     //通过IP转换获得的地理位置
	IpConnAddr        string    `json:"IpConnAddr"`     //conn连接IP地址
	ConnectionTime    time.Time `json:"connectionTime"`
	LastRequestTime   time.Time `json:"lastRequestTime"`
	LastHeartbeatTime time.Time `json:"lastHeartbeatTime"`

	mu   sync.RWMutex
	Keys map[string]any
}

// Reader 读取
func (c *Client) Reader() {
	defer func() {
		c.Hub.Disconnect <- c
	}()

	for {
		request, op, err := wsutil.ReadClientData(c.Conn)
		if err != nil {
			c.Log("xx", "Error reading data", err.Error())
			return
		}

		if op == ws.OpText {
			req := string(request)
			c.Log("<-", req)
			go Dispatcher(c, req)
		} else if op == ws.OpPing {
			err = wsutil.WriteServerMessage(c.Conn, ws.OpPong, nil)
			if err != nil {
				c.Log("xx", "Reply pong", err.Error())
			}
		} else {
			c.Log("xx", "Unrecognized action")
		}
	}
}

// Write 发送
func (c *Client) Write() {
	timer := time.NewTicker(5 * time.Second)
	defer func() {
		timer.Stop()
		c.Hub.Disconnect <- c
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				return
			}

			err := wsutil.WriteServerMessage(c.Conn, ws.OpText, msg)
			if err != nil {
				c.Log("xx", "Send msg error", err.Error())
				return
			}

			//如果设置为断开状态
			//在消息发送完成后将断开与服务器的连接
			if c.Disconnecting {
				return
			}

			c.Log("->", string(msg))
		case <-timer.C:
			err := wsutil.WriteServerMessage(c.Conn, ws.OpPing, []byte("ping"))
			if err != nil {
				c.Log("xx", "Error actively pinging the client", err.Error())
				return
			}

			c.LastHeartbeatTime = time.Now()
			if c.User != nil {
				c.User.LastHeartbeatTime = c.LastHeartbeatTime
			}
		}
	}
}

// Log websocket日志
func (c *Client) Log(symbol string, msg ...string) {
	s := strings.Join(msg, ", ")
	if c.IsLogin {
		s = fmt.Sprintf("%s %s [%s-%s] %s", c.IpAddress, symbol, c.User.Suid, c.AppId, s)
	} else {
		s = fmt.Sprintf("%s %s %s", c.IpAddress, symbol, s)
	}

	logger.SugarLog.Info(s)
}

// SendMsg 把消息加入发送队列
func (c *Client) SendMsg(msg []byte) {
	defer func() {
		if err := recover(); err != nil {
			c.Hub.Disconnect <- c
			logger.SugarLog.Errorf("SendMsg recover error(%s): %s", c.IpConnAddr, err)
		}
	}()

	c.Send <- msg
}

// SendRawMsg 构造消息再发送
func (c *Client) SendRawMsg(code int, action, msg string, data any) {
	a := &Action{
		Action: action,
		Code:   code,
		Msg:    msg,
		Data:   data,
	}

	c.SendMsg(a.Encode())
}

// Close 关闭客户端
func (c *Client) Close() {
	defer func() {
		if err := recover(); err != nil {
			c.Log("xx", "recover!! -> ", fmt.Sprintf("%v", err))
			return
		}
	}()

	if !c.Closed {
		//防止重复关闭
		c.Closed = true

		//关闭通道
		close(c.Send)

		//关闭网络连接
		_ = c.Conn.Close()

		//打印日志
		c.Log("xx", fmt.Sprintf("Close client -> %s", c.IpConnAddr))
	}
}
