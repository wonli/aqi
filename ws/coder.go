package ws

import (
	"encoding/json/v2"
	"errors"

	"github.com/gobwas/ws"
	"github.com/tidwall/gjson"
)

type Coder interface {
	Decode(data []byte) (*Request, error)
	Bind(data []byte, v any) error
	Encode(action *Action) ([]byte, error)
}

type jsonCoder struct{}

var defaultJSONCoder Coder = jsonCoder{}

func (jsonCoder) Decode(data []byte) (*Request, error) {
	result := gjson.ParseBytes(data)

	return &Request{
		Id:     result.Get("id").String(),
		Action: result.Get("action").String(),
		Params: []byte(result.Get("params").String()),
	}, nil
}

func (jsonCoder) Bind(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonCoder) Encode(action *Action) ([]byte, error) {
	return action.Encode(), nil
}

func decodeRequest(data []byte, coder Coder) (*Request, error) {
	if coder == nil {
		return nil, errors.New("websocket coder is not registered")
	}

	req, err := coder.Decode(data)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errors.New("websocket coder returned nil request")
	}

	return req, nil
}

func validateRequestFrame(op ws.OpCode, req *Request) error {
	if req == nil || req.Action == "ping" {
		return nil
	}

	r := InitManager().route(req.Action)
	if r == nil {
		if op == ws.OpBinary {
			return errors.New("binary route is not registered")
		}
		return nil
	}

	if op == ws.OpText && r.coder != nil {
		return errors.New("binary route requires a binary frame")
	}
	if op == ws.OpBinary && r.coder == nil {
		return errors.New("text route does not accept binary frames")
	}

	return nil
}

func actionFrame(action *Action) (frame, error) {
	if action == nil {
		return frame{}, errors.New("websocket action cannot be nil")
	}

	coder := InitManager().routeCoder(action.Action)
	if coder == nil {
		data, err := defaultJSONCoder.Encode(action)
		return frame{op: ws.OpText, data: data}, err
	}

	data, err := coder.Encode(action)
	return frame{op: ws.OpBinary, data: data}, err
}
