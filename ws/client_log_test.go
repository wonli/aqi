package ws

import (
	"strings"
	"testing"

	"github.com/wonli/aqi/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestClientLogPersistsPacketTraceWithoutConsoleOutput(t *testing.T) {
	fileCore, fileObserved := observer.New(zapcore.InfoLevel)
	consoleCore, consoleObserved := observer.New(zapcore.InfoLevel)

	previousFile := logger.FileLog
	previousSugar := logger.SugarLog
	logger.FileLog = zap.New(fileCore)
	logger.SugarLog = zap.New(consoleCore).Sugar()
	t.Cleanup(func() {
		logger.FileLog = previousFile
		logger.SugarLog = previousSugar
	})

	client := &Client{IpAddressPort: "127.0.0.1:1234"}
	client.Log("<-", `{"action":"bench.echo"}`)
	client.Log("->", `{"code":0}`)

	if got := consoleObserved.Len(); got != 0 {
		t.Fatalf("websocket ledger should not be emitted through console logger, got %d entries", got)
	}
	if got := fileObserved.Len(); got != 2 {
		t.Fatalf("packet traces should be persisted through file logger, got %d entries", got)
	}

	logs := client.GetRecentLogs()
	if len(logs) != 2 {
		t.Fatalf("expected packet traces in recent logs, got %d entries", len(logs))
	}
	if !strings.Contains(logs[0], "<-") || !strings.Contains(logs[1], "->") {
		t.Fatalf("recent logs did not preserve packet directions: %v", logs)
	}
}

func TestClientLogPersistsDiagnosticLogsWithoutConsoleOutput(t *testing.T) {
	fileCore, fileObserved := observer.New(zapcore.InfoLevel)
	consoleCore, consoleObserved := observer.New(zapcore.InfoLevel)

	previousFile := logger.FileLog
	previousSugar := logger.SugarLog
	logger.FileLog = zap.New(fileCore)
	logger.SugarLog = zap.New(consoleCore).Sugar()
	t.Cleanup(func() {
		logger.FileLog = previousFile
		logger.SugarLog = previousSugar
	})

	client := &Client{IpAddressPort: "127.0.0.1:1234"}
	client.Log("xx", "read failed")

	if got := consoleObserved.Len(); got != 0 {
		t.Fatalf("websocket diagnostics should not be emitted through console logger, got %d entries", got)
	}
	if got := fileObserved.Len(); got != 1 {
		t.Fatalf("diagnostic log should be persisted, got %d entries", got)
	}
	if !strings.Contains(fileObserved.All()[0].Message, "read failed") {
		t.Fatalf("unexpected diagnostic log: %q", fileObserved.All()[0].Message)
	}
}
