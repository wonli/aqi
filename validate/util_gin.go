package validate

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var GinBinding *Manager

func GinValidator() error {
	// 修改gin框架中的Validator引擎属性
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(vc.tagNameFunc)
		GinBinding = &Manager{
			Validator: v,
			Trans:     vc.getTranslator(),
		}

		return vc.registerTrans(v, GinBinding.Trans)
	}

	return nil
}
