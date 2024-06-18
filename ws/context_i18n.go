package ws

func (c *Context) i18nLoad(code int, msg string) string {
	return languageInit(c).load(code, msg)
}

func (c *Context) i18nSet(code int, msg string) {
	languageInit(c).set(code, msg)
}
