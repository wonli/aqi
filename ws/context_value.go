package ws

import (
	"reflect"
	"time"
)

type Value struct {
	data any
}

func (v *Value) By(t any) {
	if v.data == nil {
		return
	}

	tValue := reflect.ValueOf(t)
	if tValue.Kind() != reflect.Ptr || tValue.IsNil() {
		return
	}

	elem := tValue.Elem()
	dataValue := reflect.ValueOf(v.data)
	if elem.Kind() != dataValue.Kind() {
		return
	}

	elem.Set(dataValue)
}

func (v *Value) Raw() any {
	return v.data
}

func (v *Value) IsNil() bool {
	return v.data == nil
}

func (v *Value) String() string {
	if s, ok := v.data.(string); ok {
		return s
	}
	return ""
}

func (v *Value) Bool() bool {
	if b, ok := v.data.(bool); ok {
		return b
	}
	return false
}

func (v *Value) Int() int {
	if i, ok := v.data.(int); ok {
		return i
	}
	return 0
}

func (v *Value) Int8() int8 {
	if i, ok := v.data.(int8); ok {
		return i
	}
	return 0
}

func (v *Value) Int16() int16 {
	if i, ok := v.data.(int16); ok {
		return i
	}
	return 0
}

func (v *Value) Int32() int32 {
	if i, ok := v.data.(int32); ok {
		return i
	}
	return 0
}

func (v *Value) Int64() int64 {
	if i, ok := v.data.(int64); ok {
		return i
	}
	return 0
}

func (v *Value) Uint() uint {
	if i, ok := v.data.(uint); ok {
		return i
	}
	return 0
}

func (v *Value) Uint8() uint8 {
	if i, ok := v.data.(uint8); ok {
		return i
	}
	return 0
}

func (v *Value) Uint16() uint16 {
	if i, ok := v.data.(uint16); ok {
		return i
	}
	return 0
}

func (v *Value) Uint32() uint32 {
	if i, ok := v.data.(uint32); ok {
		return i
	}
	return 0
}

func (v *Value) Uint64() uint64 {
	if i, ok := v.data.(uint64); ok {
		return i
	}
	return 0
}

func (v *Value) Float32() float32 {
	if f, ok := v.data.(float32); ok {
		return f
	}
	return 0.0
}

func (v *Value) Float64() float64 {
	if f, ok := v.data.(float64); ok {
		return f
	}
	return 0.0
}

func (v *Value) Time() time.Time {
	if t, ok := v.data.(time.Time); ok {
		return t
	}

	return time.Time{}
}

func (v *Value) Duration() time.Duration {
	if t, ok := v.data.(time.Duration); ok {
		return t
	}

	return 0
}

func (v *Value) Slice() []any {
	if s, ok := v.data.([]any); ok {
		return s
	}
	return nil
}

func (v *Value) SliceString() []string {
	if s, ok := v.data.([]string); ok {
		return s
	}
	return nil
}

func (v *Value) SliceInt() []int {
	if s, ok := v.data.([]int); ok {
		return s
	}
	return nil
}

func (v *Value) Map() map[string]any {
	if m, ok := v.data.(map[string]any); ok {
		return m
	}
	return nil
}

func (v *Value) StringMap() map[string]string {
	if m, ok := v.data.(map[string]string); ok {
		return m
	}
	return nil
}
