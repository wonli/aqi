package ws

import (
	"fmt"
	"github.com/wonli/aqi/logger"
	"go.uber.org/zap"
	"time"
)

func (c *Context) AddLog(log string) {
	c.logs = append(c.logs, log)
}

func (c *Context) FlushLog() {
	if c.logs == nil || len(c.logs) == 0 {
		return
	}

	clientInfo := fmt.Sprintf("action(%s), reqIP(%s), reqAt(%s), connectAt(%s), clientId(%s)",
		c.Action,
		c.Client.IpAddressPort,
		c.Client.LastRequestTime.Format(time.RFC3339),
		c.Client.ConnectionTime.Format(time.RFC3339),
		c.Client.ClientId,
	)

	runtimeLogs := []zap.Field{
		zap.String("", clientInfo),
	}

	for _, log := range c.logs {
		runtimeLogs = append(runtimeLogs, zap.String("", log))
	}

	logger.RuntimeLog.Info("runtime", runtimeLogs...)
}
