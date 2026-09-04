package ws

import (
	"time"

	"github.com/tidwall/gjson"
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

	t := time.Now()
	if req.Action == "ping" {
		c.SetLastHeartbeat(t)
		c.SendActionMsg(&Action{Action: "ping", Msg: "pong"})
		return
	}

	user, _, _ := c.LoginState()
	if user != nil {
		isBanned, bandTime := user.IsBanned()
		if isBanned {
			c.SendActionMsg(&Action{Action: "sys.ban", Code: -1001, Data: bandTime})
			return
		}
	}

	c.TouchRequest(t)

	handlers := InitManager().Handlers(req.Action)
	if len(handlers) == 0 {
		c.SendActionMsg(&Action{Action: req.Action, Code: -1005, Msg: "request not supported"})
		return
	}

	defaultLanguage := "zh"
	if wss != nil {
		defaultLanguage = wss.DefaultLanguage()
	}
	language := c.Language()
	if language == "" {
		language = defaultLanguage
	}

	ctx := &Context{
		Id:     req.Id,
		Params: req.Params,
		Action: req.Action,

		Client: c,
		Server: wss,

		handlers: handlers,
		ctx:      c.Context(),

		language:   language,
		defaultLng: defaultLanguage,
	}

	defer ctx.FlushLog()

	ctx.handlers[0](ctx)
	ctx.Next()
}
