package ws

import "strings"

const clientLanguageKey = "_aqi_language"

// SetLanguage 设置当前连接使用的语言。
func (c *Client) SetLanguage(language string) {
	if c == nil {
		return
	}
	language = strings.TrimSpace(language)
	c.mu.Lock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}
	if language == "" {
		delete(c.Keys, clientLanguageKey)
	} else {
		c.Keys[clientLanguageKey] = language
	}
	c.mu.Unlock()
}

// Language 返回当前连接使用的语言。
func (c *Client) Language() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	language, _ := c.Keys[clientLanguageKey].(string)
	return language
}
