package ws

import (
	"math"
)

type Context struct {
	Client   *Client
	Action   string
	Params   string
	Response *Action
	Server   *Server

	index    int8
	handlers HandlersChain

	logs []string

	language   string
	defaultLng string
}

const abortIndex int8 = math.MaxInt8 / 2

func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort 放弃调用后续方法
func (c *Context) Abort() {
	c.index = abortIndex
}
