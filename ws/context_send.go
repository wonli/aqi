package ws

// Send 发送数据给用户
func (c *Context) Send(data any) {
	msg := New(c.Action).WithData(data)

	c.Response = msg
	c.Client.SendMsg(msg.Encode())
}

// SendOk 发送成功消息
func (c *Context) SendOk() {
	msg := New(c.Action)

	c.Response = msg
	c.Client.SendMsg(msg.Encode())
}

// SendCode 发送状态消息
func (c *Context) SendCode(code int, msg string) {
	//开发模式下更新默认语言文件
	if c.Server.isDev && c.language == c.defaultLng {
		c.i18nSet(code, msg)
	}

	if c.language != c.defaultLng {
		translate := c.i18nLoad(code, msg)
		if translate != "" {
			msg = translate
		}
	}

	m := New(c.Action).WithCode(code).WithMsg(msg)

	c.Response = m
	c.Client.SendMsg(m.Encode())
}

// SendMsg 发送消息给当前用户
func (c *Context) SendMsg(msg string) {
	m := New(c.Action).WithMsg(msg)

	c.Response = m
	c.Client.SendMsg(m.Encode())
}

// SendActionData 发送数据给当前用户
func (c *Context) SendActionData(action string, data any) {
	m := New(action).WithData(data)

	c.Response = m
	c.Client.SendMsg(m.Encode())
}

// SendActionMsg 发送消息给当前用户
func (c *Context) SendActionMsg(action, msg string) {
	m := New(action).WithMsg(msg)

	c.Response = m
	c.Client.SendMsg(m.Encode())
}

// SendTo 发送给指定用户
func (c *Context) SendTo(uid, action string, data any) {
	m := New(action).WithData(data)
	c.Response = m

	user := c.Client.Hub.User(uid)
	if user != nil {
		user.SendMsg(m.Encode())
	}
}

// SendToApp 发送消息给指定的app
func (c *Context) SendToApp(appId string, msg *Action) {
	c.Response = msg
	if c.Client.User != nil {
		c.Client.User.SendMsgToApp(appId, msg.Encode())
	}
}

// SendToApps 发送RAW消息给当前用户所有客户端
func (c *Context) SendToApps(msg *Action) {
	c.Response = msg
	if c.Client.User != nil {
		c.Client.User.SendMsg(msg.Encode())
	} else {
		c.Client.SendMsg(msg.Encode())
	}
}

// SendRawTo 发送RAW消息给指定用户
func (c *Context) SendRawTo(uid string, msg *Action) {
	c.Response = msg
	user := c.Client.Hub.User(uid)
	if user != nil {
		user.SendMsg(msg.Encode())
	}
}

// Broadcast 发送广播
func (c *Context) Broadcast(msg *Action) {
	c.Response = msg
	c.Client.Hub.Broadcast(msg.Encode())
}
