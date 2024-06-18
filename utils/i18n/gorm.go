package i18n

import (
	"database/sql/driver"
	"fmt"
)

type GType struct {
	Data any
	lng  string
}

func NewGType(data any, lng string) *GType {
	return &GType{Data: data, lng: lng}
}

func (i *GType) GormDataType() string {
	return "string"
}

func (i *GType) Scan(value interface{}) error {
	v, ok := value.([]byte)
	if ok {
		i.Data = string(v)
		return nil
	}

	return nil
}

func (i *GType) String() string {
	return fmt.Sprintf("%s", i.Data)
}

func (i *GType) Value() (driver.Value, error) {
	return driver.Value(i.Data), nil
}
