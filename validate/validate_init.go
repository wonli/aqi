package validate

import (
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	"github.com/go-playground/validator/v10"

	ut "github.com/go-playground/universal-translator"
	enT "github.com/go-playground/validator/v10/translations/en"
	zhT "github.com/go-playground/validator/v10/translations/zh"
)

var sg sync.Once
var vc *validatorConfig

type validatorConfig struct {
	locale string
	zh     locales.Translator
	en     locales.Translator
}

// InitTranslator validator默认仅支持中英文
func InitTranslator(locale string) *validatorConfig {
	sg.Do(func() {
		zhl := zh.New() // 中文翻译器
		enl := en.New() // 英文翻译器

		//赋值给valid
		vc = &validatorConfig{
			locale: locale,
			zh:     zhl,
			en:     enl,
		}
	})

	return vc
}

// 处理字段名称
// 中文使用label标签，其他语言label+语言名称，没有设置时使用json名称
func (a *validatorConfig) tagNameFunc(fld reflect.StructField) string {
	var name string
	switch a.locale {
	case "zh":
		name = fld.Tag.Get("label")
	default:
		name = fld.Tag.Get("label_" + a.locale)
		if name == "" {
			name = strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
		}
	}

	return name
}

// translator
func (a *validatorConfig) getTranslator() ut.Translator {
	// 第一个参数是备用（fallback）的语言环境
	// 后面的参数是应该支持的语言环境（支持多个）
	// uni := ut.New(zhl, zhl) 也是可以的
	uni := ut.New(a.en, a.zh, a.en)

	// locale 通常取决于 http 请求头的 'Accept-Language'
	// 也可以使用 uni.FindTranslator(...) 传入多个locale进行查找
	trans, ok := uni.GetTranslator(a.locale)
	if !ok {
		color.Red("uni.GetTranslator(%s) failed", a.locale)
		os.Exit(0)
	}

	return trans
}

// registerTrans
func (a *validatorConfig) registerTrans(v *validator.Validate, trans ut.Translator) error {
	var err error
	switch a.locale {
	case "en":
		err = enT.RegisterDefaultTranslations(v, trans)
	case "zh":
		err = zhT.RegisterDefaultTranslations(v, trans)
	default:
		err = enT.RegisterDefaultTranslations(v, trans)
	}

	return err
}
