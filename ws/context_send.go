package ws

func (c *Context) send(msg *Action) {
	c.Response = msg
	c.Client.SendActionMsg(msg)
}

// Send 发送数据给用户
func (c *Context) Send(data any) {
	c.send(New(c.Action).WithId(c.Id).WithData(data))
}

// SendOk 发送成功消息
func (c *Context) SendOk() {
	c.send(New(c.Action).WithId(c.Id))
}

// SendCode 发送状态消息
func (c *Context) SendCode(code int, msg string) {
	msg = c.i18nLoad(code, msg)
	c.send(New(c.Action).WithId(c.Id).WithCode(code).WithMsg(msg))
}

// SendMsg 发送消息给当前用户
func (c *Context) SendMsg(msg string) {
	c.send(New(c.Action).WithId(c.Id).WithMsg(msg))
}

// SendAction 发送Action
func (c *Context) SendAction(msg *Action) {
	c.send(msg)
}

// SendActionData 发送数据给当前用户
func (c *Context) SendActionData(action string, data any) {
	c.send(New(action).WithData(data))
}

// SendActionMsg 发送消息给当前用户
func (c *Context) SendActionMsg(action, msg string) {
	c.send(New(action).WithMsg(msg))
}

// SendTo 发送给指定用户
func (c *Context) SendTo(uid, action string, data any) {
	msg := New(action).WithData(data)
	c.Response = msg

	user := c.Client.Hub.User(uid)
	if user != nil {
		user.sendAction(msg)
	}
}

// SendToApp 发送消息给指定的app
func (c *Context) SendToApp(appId string, msg *Action) {
	c.Response = msg
	if c.Client.User != nil {
		c.Client.User.sendActionToApp(appId, msg)
	}
}

// SendToApps 发送消息给当前用户所有客户端
func (c *Context) SendToApps(msg *Action) {
	c.Response = msg
	if c.Client.User != nil {
		c.Client.User.sendAction(msg)
	} else {
		c.Client.SendActionMsg(msg)
	}
}

// SendRawTo 发送RAW消息给指定用户
func (c *Context) SendRawTo(uid string, msg *Action) {
	c.Response = msg
	user := c.Client.Hub.User(uid)
	if user != nil {
		user.sendAction(msg)
	}
}

// Broadcast 发送广播
func (c *Context) Broadcast(msg *Action) {
	c.Response = msg
	c.Client.Hub.broadcastAction(msg)
}
