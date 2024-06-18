package ws

import (
	"fmt"
)

var (
	ErrServerError          = NewError(sys, 1100, "Server under maintenance")
	ErrUncertified          = NewError(sys, 1120, "Please log in first")
	ErrCertificationExpired = NewError(sys, 1121, "Login has expired")
	ErrJwtBLOCKED           = NewError(sys, 1123, "Login has expired")
	ErrParamsInvalid        = NewError(sys, 1131, "Invalid parameters")
	ErrAuthentic            = NewError(sys, 1141, "Please login first")
)

type Error ApiData

var codes = map[int]string{}

func NewError(appId Appid, code int, msg string) *Error {
	if appId != sys {
		a := int(appId)
		if a < minAppid || a > maxAppid {
			panic(fmt.Sprintf("error AppId %d", appId))
		}

		if code < minCode || code > maxCode {
			panic(fmt.Sprintf("error code %d", code))
		}

		code = a*base + code
	}

	if _, ok := codes[code]; ok {
		panic(fmt.Sprintf("Error code %d already exists, please choose another one", code))
	}

	codes[code] = msg
	return &Error{Code: code, Msg: msg}
}

// WithMsg 覆盖业务错误提示内容
func (e *Error) WithMsg(msg string) *Error {
	e.Msg = msg
	return e
}

// WithError 兼容Binding错误码及多语言翻译
// 使用前需要调用 validate.GinValidator() 初始化
// 字段中文名称使用 `label:"名称"` 指定
func (e *Error) WithError(err error) *Error {
	if err == nil {
		return e
	}

	e.Msg = BindingErrors(err).Error()
	return e
}

// WithHttpStatus 处理HTTP状态码
func (e *Error) WithHttpStatus(status int) *Error {
	e.HttpStatus = status
	return e
}
