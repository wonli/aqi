package ws

import (
	"time"

	"github.com/tidwall/gjson"
)

func Dispatcher(c *Client, request string) {
	t := time.Now()
	//ping直接回应
	action := gjson.Get(request, "action").String()
	if action == "ping" {
		c.LastHeartbeatTime = t
		c.SendRawMsg(0, "ping", "pong", nil)
		return
	}

	//是否被禁言
	if c.User != nil {
		isBanned, bandTime := c.User.IsBanned()
		if isBanned {
			c.SendRawMsg(-11, "sys.ban", "You have been ban", bandTime)
			return
		}
	}

	//请求频率限制5毫秒
	if t.Sub(c.LastRequestTime).Microseconds() <= 2 {
		c.SendRawMsg(-13, "sys.requestLimit", "Your requests are too frequent", nil)
		return
	} else {
		//更新最后请求时间
		c.LastRequestTime = t

		//如果心跳时间为0，设置为当前时间
		//防止在连接瞬间被哨兵扫描而断开
		if c.LastHeartbeatTime.IsZero() {
			c.LastHeartbeatTime = t
		}
	}

	handlers := InitManager().Handlers(action)
	if handlers == nil || len(handlers) == 0 {
		c.SendRawMsg(-15, action, "Request not supported", nil)
		return
	}

	ctx := &Context{
		Client: c,
		Action: action,
		Params: gjson.Get(request, "params").String(),
		Server: wss,

		handlers: handlers,

		language:   "zh",
		defaultLng: "zh",
	}

	defer ctx.FlushLog()

	ctx.handlers[0](ctx)
	ctx.Next()
}
