package validate

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var normalManagers sync.Map

func Normal(language string) *Manager {
	language = normalizeLocale(language)
	if value, ok := normalManagers.Load(language); ok {
		return value.(*Manager)
	}

	cc := newValidatorConfig(language)
	validate := validator.New()
	validate.RegisterTagNameFunc(cc.tagNameFunc)

	translator := cc.getTranslator()
	if err := cc.registerTrans(validate, translator); err != nil {
		panic(err)
	}

	manager := &Manager{
		Validator: validate,
		Trans:     translator,
	}
	actual, _ := normalManagers.LoadOrStore(language, manager)
	return actual.(*Manager)
}
