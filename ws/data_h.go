package ws

import (
	"encoding/json"

	"go.uber.org/zap"

	"github.com/wonli/aqi/logger"
)

// H 类似gin.H
type H map[string]any

func (h *H) Get(key string) (any, bool) {
	if *h == nil {
		return nil, false
	}

	val, ok := (*h)[key]
	return val, ok
}

func (h *H) Set(key string, val any) {
	if *h == nil {
		*h = make(map[string]any)
	}

	(*h)[key] = val
}

func (h *H) Unmarshal(v any) error {
	d, err := json.Marshal(h)
	if err != nil {
		logger.SugarLog.Error("构造参数失败",
			zap.String("error", err.Error()),
		)
		return err
	}

	err = json.Unmarshal(d, v)
	if err != nil {
		logger.SugarLog.Error("解析参数失败",
			zap.String("error", err.Error()),
		)
		return err
	}

	return nil
}

func (h *H) Marshal() []byte {
	d, err := json.Marshal(h)
	if err != nil {
		logger.SugarLog.Error("构造参数失败",
			zap.String("error", err.Error()),
		)

		return d
	}

	return d
}
