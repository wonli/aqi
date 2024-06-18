package ws

import (
	"encoding/json"

	"go.uber.org/zap"

	"github.com/wonli/aqi/logger"
)

// Action Websocket通讯协议
type Action struct {
	Action string `json:"action"`

	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
	Data any    `json:"data,omitempty"`
}

func (m *Action) Encode() []byte {
	return m.json()
}

// JSON 格式化
func (m *Action) json() []byte {
	r, err := json.Marshal(m)
	if err != nil {
		logger.SugarLog.Error("JSON格式化失败",
			zap.String("error", err.Error()),
		)
	}

	return r
}
