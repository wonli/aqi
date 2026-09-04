package validate

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var GinBinding *Manager

func GinValidator() error {
	// 修改gin框架中的Validator引擎属性
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		config := defaultValidatorConfig()
		v.RegisterTagNameFunc(config.tagNameFunc)
		GinBinding = &Manager{
			Validator: v,
			Trans:     config.getTranslator(),
		}

		return config.registerTrans(v, GinBinding.Trans)
	}

	return nil
}
