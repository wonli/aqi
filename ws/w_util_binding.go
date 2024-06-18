package ws

import "github.com/wonli/aqi/validate"

// BindingErrors 处理错误信息
func BindingErrors(e error) error {
	if validate.GinBinding == nil {
		return e
	}

	return validate.GinBinding.Translator(e)
}
