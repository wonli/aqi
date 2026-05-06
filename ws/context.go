package ws

import (
	"context"
	"math"
)

type Context struct {
	Id     string
	Action string
	Params string

	Client   *Client
	Response *Action
	Server   *Server

	index    int8
	handlers HandlersChain

	ctx context.Context

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

func (c *Context) Context() context.Context {
	if c == nil || c.ctx == nil {
		return context.Background()
	}

	return c.ctx
}

func (c *Context) WithContext(ctx context.Context) {
	if c == nil {
		return
	}

	if ctx == nil {
		c.ctx = context.Background()
		return
	}

	c.ctx = ctx
}
