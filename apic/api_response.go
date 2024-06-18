package apic

import (
	"encoding/json"
	"net/http"
)

type ResponseData struct {
	HttpStatus int         `json:"http_status"`
	Header     http.Header `json:"header,omitempty"`
	Data       []byte      `json:"data,omitempty"`
	Text       string      `json:"text,omitempty"`
}

func (a *ResponseData) MarshalToString() (string, error) {
	return marshal(a)
}

func (a *ResponseData) BindJson(d any) error {
	err := json.Unmarshal(a.Data, d)
	if err != nil {
		return err
	}

	return nil
}
