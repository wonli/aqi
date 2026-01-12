package ws

// Pub 发布消息到主题
func (c *Context) Pub(topicId string, data any) {
	c.Client.Hub.PubSub.Pub(topicId, data)
}

// Sub 订阅主题（当前用户）
func (c *Context) Sub(topicId string) {
	if c.Client.User != nil {
		c.Client.Hub.PubSub.Sub(topicId, c.Client.User)
	}
}

// SubFunc 以函数方式订阅主题
func (c *Context) SubFunc(topicId string, f func(msg *TopicMsg)) {
	c.Client.Hub.PubSub.SubFunc(topicId, f)
}

// Unsub 取消订阅主题（当前用户）
func (c *Context) Unsub(topicId string) {
	if c.Client.User != nil {
		c.Client.Hub.PubSub.Unsub(topicId, c.Client.User)
	}
}
