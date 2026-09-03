package validate

import (
	"errors"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestValidatorConfigTagNameFunc(t *testing.T) {
	type sample struct {
		Name    string `json:"name" label:"姓名" label_en:"Name"`
		Fallback string `json:"fallback,omitempty"`
		Ignored string `json:"-"`
	}

	typ := reflect.TypeOf(sample{})

	zh := &ValidatorConfig{locale: "zh"}
	if got := zh.tagNameFunc(typ.Field(0)); got != "姓名" {
		t.Fatalf("zh tagNameFunc() = %q, want %q", got, "姓名")
	}

	en := &ValidatorConfig{locale: "en"}
	if got := en.tagNameFunc(typ.Field(0)); got != "Name" {
		t.Fatalf("en tagNameFunc() = %q, want %q", got, "Name")
	}
	if got := en.tagNameFunc(typ.Field(1)); got != "fallback" {
		t.Fatalf("fallback tagNameFunc() = %q, want %q", got, "fallback")
	}
	if got := en.tagNameFunc(typ.Field(2)); got != "" {
		t.Fatalf("ignored tagNameFunc() = %q, want empty", got)
	}
}

func TestManagerValidateAndTranslate(t *testing.T) {
	manager := Normal("en")

	type request struct {
		Name string `json:"name" label_en:"Name" validate:"required"`
	}

	if err := manager.Validate(request{Name: "aqi"}); err != nil {
		t.Fatalf("Validate(valid) returned error: %v", err)
	}
	if err := manager.Validate(request{}); err == nil {
		t.Fatal("Validate(invalid) expected error, got nil")
	}

	sentinel := errors.New("sentinel")
	if got := manager.Translator(sentinel); !errors.Is(got, sentinel) {
		t.Fatalf("Translator(non-validation error) = %v, want sentinel", got)
	}
}

func TestManagerRegisterValidator(t *testing.T) {
	manager := Normal("en")
	const tag = "aqi_nonempty_test"

	err := manager.RegisterValidator(tag, "must not be empty", func(fl validator.FieldLevel) bool {
		return fl.Field().String() != ""
	})
	if err != nil {
		t.Fatalf("RegisterValidator returned error: %v", err)
	}

	type request struct {
		Value string `validate:"aqi_nonempty_test"`
	}
	if err := manager.Validate(request{Value: "ok"}); err != nil {
		t.Fatalf("custom validator rejected valid value: %v", err)
	}
	if err := manager.Validate(request{}); err == nil {
		t.Fatal("custom validator accepted empty value")
	}
}
