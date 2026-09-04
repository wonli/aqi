package ws

func (c *Context) i18nLoad(code int, msg string) string {
	if c == nil || c.Server == nil {
		return msg
	}
	return c.Server.translate(c.Language(), c.Action, code, msg)
}
