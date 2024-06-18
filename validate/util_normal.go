package validate

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var once sync.Once
var normalManager *Manager

func Normal(language string) *Manager {
	once.Do(func() {
		cc := InitTranslator(language)

		validate := validator.New()
		validate.RegisterTagNameFunc(cc.tagNameFunc)

		translator := cc.getTranslator()
		err := cc.registerTrans(validate, translator)
		if err != nil {
			panic(err)
		}

		normalManager = &Manager{
			Validator: validate,
			Trans:     translator,
		}
	})

	return normalManager
}
