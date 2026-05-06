package middlewares

import (
	"fmt"
	"runtime/debug"

	"github.com/wonli/aqi/logger"
	"github.com/wonli/aqi/telemetry"
	"github.com/wonli/aqi/ws"
)

func Recovery() ws.HandlerFunc {
	return func(a *ws.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取 panic 发生的堆栈跟踪
				stack := debug.Stack()
				if logger.SugarLog != nil {
					logger.SugarLog.Errorf("Panic happened: %s \n %s\n", err, stack)
				}
				panicErr := fmt.Errorf("panic: %v", err)
				span := telemetry.SpanFromContext(a.Context())
				span.RecordError(panicErr)
				span.SetStatus(telemetry.StatusError, panicErr.Error())

				a.SendCode(-30, "服务维护中")
				a.Abort()
			}
		}()

		a.Next()
	}
}
