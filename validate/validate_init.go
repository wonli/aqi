package validate

import (
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	"github.com/go-playground/validator/v10"

	ut "github.com/go-playground/universal-translator"
	enT "github.com/go-playground/validator/v10/translations/en"
	zhT "github.com/go-playground/validator/v10/translations/zh"
)

var (
	vcMu sync.RWMutex
	vc   *ValidatorConfig
)

type ValidatorConfig struct {
	locale string
	zh     locales.Translator
	en     locales.Translator
}

// InitTranslator validator默认仅支持中英文
func InitTranslator(locale string) *ValidatorConfig {
	config := newValidatorConfig(locale)
	vcMu.Lock()
	vc = config
	vcMu.Unlock()
	return config
}

func newValidatorConfig(locale string) *ValidatorConfig {
	locale = normalizeLocale(locale)
	return &ValidatorConfig{
		locale: locale,
		zh:     zh.New(),
		en:     en.New(),
	}
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(strings.ToLower(locale))
	if i := strings.IndexAny(locale, "-_"); i > 0 {
		locale = locale[:i]
	}
	if locale != "zh" && locale != "en" {
		return "en"
	}
	return locale
}

func defaultValidatorConfig() *ValidatorConfig {
	vcMu.RLock()
	config := vc
	vcMu.RUnlock()
	if config != nil {
		return config
	}
	return InitTranslator("zh")
}

// 处理字段名称
// 中文使用label标签，其他语言label+语言名称，没有设置时使用json名称
func (a *ValidatorConfig) tagNameFunc(fld reflect.StructField) string {
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
func (a *ValidatorConfig) getTranslator() ut.Translator {
	// 第一个参数是备用（fallback）的语言环境
	// 后面的参数是应该支持的语言环境（支持多个）
	uni := ut.New(a.en, a.zh, a.en)
	trans, ok := uni.GetTranslator(a.locale)
	if ok {
		return trans
	}
	trans, _ = uni.GetTranslator("en")
	return trans
}

// registerTrans
func (a *ValidatorConfig) registerTrans(v *validator.Validate, trans ut.Translator) error {
	var err error
	switch a.locale {
	case "zh":
		err = zhT.RegisterDefaultTranslations(v, trans)
	default:
		err = enT.RegisterDefaultTranslations(v, trans)
	}

	return err
}
