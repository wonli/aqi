package apic

import (
	"encoding/json"
)

// Params map[string]any
type Params map[string]any

func (p Params) With(key string, val any) Params {
	p[key] = val
	return p
}

func (p Params) WithParams(params map[string]any) Params {
	for key, val := range params {
		p[key] = val
	}

	return p
}

func (p Params) Marshal() []byte {
	bytes, err := json.Marshal(p)
	if err != nil {
		return nil
	}

	return bytes
}
