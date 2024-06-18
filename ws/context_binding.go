package ws

import (
	"encoding/json"

	"github.com/wonli/aqi/validate"
)

func (c *Context) BindingJson(s any) error {
	err := json.Unmarshal([]byte(c.Params), s)
	if err != nil {
		return err
	}

	return nil
}

func (c *Context) BindingJsonPath(s any, path string) error {
	data := c.Get(path)
	err := json.Unmarshal([]byte(data), s)
	if err != nil {
		return err
	}

	return nil
}

func (c *Context) BindingValidateJson(s any) error {
	err := json.Unmarshal([]byte(c.Params), s)
	if err != nil {
		return err
	}

	err = validate.Normal(c.language).Validate(s)
	if err != nil {
		return err
	}

	return nil
}
