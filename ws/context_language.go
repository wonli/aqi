package ws

import "strings"

// Language 返回当前请求使用的语言。
func (c *Context) Language() string {
	if c == nil || c.language == "" {
		if c != nil && c.defaultLng != "" {
			return c.defaultLng
		}
		return "zh"
	}
	return c.language
}

// SetLanguage 切换当前请求以及后续连接请求使用的语言。
func (c *Context) SetLanguage(language string) {
	if c == nil {
		return
	}
	language = strings.TrimSpace(language)
	if language == "" {
		language = c.defaultLng
	}
	c.language = language
	if c.Client != nil {
		c.Client.SetLanguage(language)
	}
}
