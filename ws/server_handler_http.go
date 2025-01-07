package ws

import (
	"net"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"go.uber.org/zap"

	"github.com/wonli/aqi/logger"
	"github.com/wonli/aqi/utils/ip"
)

func HttpHandler(w http.ResponseWriter, r *http.Request) {
	u := ws.HTTPUpgrader{
		Protocol: func(s string) bool {
			return true
		},
	}

	conn, _, h, err := u.Upgrade(r, w)
	if err != nil {
		logger.SugarLog.Error("UpgradeHTTP",
			zap.String("error", err.Error()),
		)
		return
	}

	if h.Protocol != "" {
		r.Header.Set("Sec-Websocket-Protocol", h.Protocol)
	}

	addr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		logger.SugarLog.Errorf("获取IP地址错误")
		return
	}

	c := &Client{
		Hub:            Hub,
		Conn:           conn,
		Send:           make(chan []byte, 32),
		IpAddress:      ip.GetIPAddress(r),
		IpConnAddr:     addr.String(),
		ConnectionTime: time.Now(),
		HttpRequest:    r,
		HttpWriter:     w,
	}

	c.Hub.Connection <- c
	go c.Reader()
	go c.Write()

}
