package ws

import (
	"github.com/tidwall/gjson"
	"time"
)

func Dispatcher(c *Client, request string) {
	var req struct {
		Id     string `json:"id"`
		Action string `json:"action"`
		Params string `json:"params"`
	}

	result := gjson.Parse(request)
	req.Id = result.Get("id").String()
	req.Params = result.Get("params").String()
	req.Action = result.Get("action").String()

	//ping直接回应
	t := time.Now()
	if req.Action == "ping" {
		c.LastHeartbeatTime = t
		c.SendActionMsg(&Action{Action: "ping", Msg: "pong"})
		return
	}

	//是否被禁言
	if c.User != nil {
		isBanned, bandTime := c.User.IsBanned()
		if isBanned {
			c.SendActionMsg(&Action{Action: "sys.ban", Code: -1001, Data: bandTime})
			return
		}
	}

	//更新最后请求时间
	c.LastRequestTime = t

	//如果心跳时间为0，设置为当前时间
	//防止在连接瞬间被哨兵扫描而断开
	if c.LastHeartbeatTime.IsZero() {
		c.LastHeartbeatTime = t
	}

	handlers := InitManager().Handlers(req.Action)
	if handlers == nil || len(handlers) == 0 {
		c.SendActionMsg(&Action{Action: req.Action, Code: -1005, Msg: "request not supported"})
		return
	}

	ctx := &Context{
		Id:     req.Id,
		Params: req.Params,
		Action: req.Action,

		Client: c,
		Server: wss,

		handlers: handlers,

		language:   "zh",
		defaultLng: "zh",
	}

	defer ctx.FlushLog()

	ctx.handlers[0](ctx)
	ctx.Next()
}
