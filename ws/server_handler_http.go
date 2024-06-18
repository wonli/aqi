package ws

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"go.uber.org/zap"

	"github.com/wonli/aqi/logger"
)

func getRealIP(r *http.Request) string {
	// Check if behind a proxy
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// This header can contain multiple IPs separated by comma
		// The first one is the original IP
		parts := strings.Split(xForwardedFor, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		return parts[0] // Return the first IP which is the client's real IP
	}

	// If the X-Real-IP header is set, then use it
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// Fallback to using RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // In case there was an issue parsing, just return the whole thing
	}

	return ip
}

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
		IpAddress:      getRealIP(r),
		IpConnAddr:     addr.String(),
		ConnectionTime: time.Now(),
		Endpoint:       r.RequestURI,
	}

	c.Hub.Connection <- c
	go c.Reader()
	go c.Write()

}
