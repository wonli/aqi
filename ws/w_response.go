package ws

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Ctx *ApiData

	g        *gin.Context
	httpCode int
}

func (r *Response) WithData(data any) *Response {
	r.Ctx.Data = data
	return r
}

func (r *Response) WithError(e Error, err error) *Response {
	r.Ctx.Code = e.Code
	if err != nil {
		r.Ctx.Msg = fmt.Sprintf("%s,%s", e.Msg, err.Error())
	} else {
		r.Ctx.Msg = e.Msg
	}

	return r
}

func (r *Response) WithMsg(msg string) *Response {
	r.Ctx.Msg = msg
	return r
}

func (r *Response) Send() {
	r.g.JSON(r.httpCode, r.Ctx)
}

func NewResponse(g *gin.Context, httpCode int) *Response {
	return &Response{
		g:        g,
		httpCode: httpCode,
		Ctx:      &ApiData{},
	}
}
