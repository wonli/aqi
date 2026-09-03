package ws

import (
	"strings"
	"testing"

	"github.com/wonli/aqi/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestClientLogKeepsPacketTraceInMemoryWithoutLoggerOutput(t *testing.T) {
	core, observed := observer.New(zapcore.InfoLevel)
	previous := logger.SugarLog
	logger.SugarLog = zap.New(core).Sugar()
	t.Cleanup(func() {
		logger.SugarLog = previous
	})

	client := &Client{IpAddressPort: "127.0.0.1:1234"}
	client.Log("<-", `{"action":"bench.echo"}`)
	client.Log("->", `{"code":0}`)

	if got := observed.Len(); got != 0 {
		t.Fatalf("packet traces should not be emitted through logger, got %d entries", got)
	}

	logs := client.GetRecentLogs()
	if len(logs) != 2 {
		t.Fatalf("expected packet traces in recent logs, got %d entries", len(logs))
	}
	if !strings.Contains(logs[0], "<-") || !strings.Contains(logs[1], "->") {
		t.Fatalf("recent logs did not preserve packet directions: %v", logs)
	}
}

func TestClientLogStillEmitsDiagnosticLogs(t *testing.T) {
	core, observed := observer.New(zapcore.InfoLevel)
	previous := logger.SugarLog
	logger.SugarLog = zap.New(core).Sugar()
	t.Cleanup(func() {
		logger.SugarLog = previous
	})

	client := &Client{IpAddressPort: "127.0.0.1:1234"}
	client.Log("xx", "read failed")

	if got := observed.Len(); got != 1 {
		t.Fatalf("diagnostic logs should still be emitted, got %d entries", got)
	}
	if !strings.Contains(observed.All()[0].Message, "read failed") {
		t.Fatalf("unexpected diagnostic log: %q", observed.All()[0].Message)
	}
}
