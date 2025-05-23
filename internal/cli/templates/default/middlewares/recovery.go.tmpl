package middlewares

import (
	"github.com/wonli/aqi/logger"
	"github.com/wonli/aqi/ws"
	"runtime/debug"
)

func Recovery() ws.HandlerFunc {
	return func(a *ws.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取 panic 发生的堆栈跟踪
				stack := debug.Stack()
				logger.SugarLog.Errorf("Panic happened: %s \n %s\n", err, stack)

				a.SendCode(30, "服务维护中")
				a.Abort()
			}
		}()

		a.Next()
	}
}
