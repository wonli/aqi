package validate

import (
	"fmt"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

type Manager struct {
	//Trans
	Trans ut.Translator

	//允许外部自定义验证方法
	Validator *validator.Validate
}

// RegisterValidator 自定义简单验证方法
func (g *Manager) RegisterValidator(tag, errMsg string, fn validator.Func) error {
	err := g.Validator.RegisterValidation(tag, fn)
	if err != nil {
		return err
	}

	return g.Validator.RegisterTranslation(tag, g.Trans,
		func(ut ut.Translator) error {
			return ut.Add(tag, errMsg, true)
		},

		func(ut ut.Translator, fe validator.FieldError) string {
			t, _ := ut.T(tag, fe.Tag())
			return t
		},
	)
}

// RegisterValidatorFunc 自定义方法封装
func (g *Manager) RegisterValidatorFunc(tag string,
	fn validator.Func, rFn validator.RegisterTranslationsFunc, tFn validator.TranslationFunc) error {
	err := g.Validator.RegisterValidation(tag, fn)
	if err != nil {
		return err
	}

	return g.Validator.RegisterTranslation(tag, g.Trans, rFn, tFn)
}

// Translator 语言翻译
func (g *Manager) Translator(e error) error {
	errs, ok := e.(validator.ValidationErrors)
	if !ok {
		return e
	}

	if g.Trans == nil {
		return errs
	}

	errorsTranslations := errs.Translate(g.Trans)
	for _, err := range errs {
		namespace := err.Namespace()
		if s, ok := errorsTranslations[namespace]; ok {
			return fmt.Errorf(s)
		}
	}

	return errs
}

// Validate 执行验证并翻译配置指定的语言
func (g *Manager) Validate(dataStruct any) error {
	//处理数据
	err := g.Validator.Struct(dataStruct)
	if err != nil {
		return g.Translator(err)
	}

	return nil
}
