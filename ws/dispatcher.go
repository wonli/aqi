package ws

import "time"

func dispatcher(c *Client, req *Request) {
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

	r := InitManager().route(req.Action)
	if r == nil || len(r.handlers) == 0 {
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
		Params: string(req.Params),
		Action: req.Action,

		Client: c,
		Server: wss,

		request: req,
		route:   r,

		handlers: r.handlers,
		ctx:      c.Context(),

		language:   language,
		defaultLng: defaultLanguage,
	}

	defer ctx.FlushLog()

	ctx.handlers[0](ctx)
	ctx.Next()
}
