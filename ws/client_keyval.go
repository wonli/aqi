package ws

func (c *Client) SetKey(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}

	c.Keys[key] = value
}

func (c *Client) GetKey(key string) *Value {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.Keys[key]
	if !exists {
		return &Value{}
	}

	return &Value{data: value}
}
