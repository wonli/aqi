package utils

import (
	"reflect"
)

func StructToMap(input any) map[string]any {
	out := make(map[string]any)
	v := reflect.ValueOf(input)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// we only accept structs
	if v.Kind() != reflect.Struct {
		return nil
	}

	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fi := typ.Field(i)
		// skip unexported fields
		if fi.PkgPath != "" {
			continue
		}

		out[fi.Name] = v.Field(i).Interface()
	}
	return out
}
