package ws

import (
	"encoding/json/v2"

	"github.com/wonli/aqi/validate"
)

func (c *Context) Bind(s any) error {
	coder := defaultJSONCoder
	var params []byte

	if c.request != nil {
		params = c.request.Params
	} else {
		params = []byte(c.Params)
	}
	if c.route != nil && c.route.coder != nil {
		coder = c.route.coder
	}

	return coder.Bind(params, s)
}

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

	err = validate.Normal(c.Language()).Validate(s)
	if err != nil {
		return err
	}

	return nil
}
