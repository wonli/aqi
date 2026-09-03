package ws

import (
	"sync/atomic"

	"go.uber.org/zap"
)

const websocketLogQueueSize = 65536

type websocketLogEntry struct {
	logger  *zap.Logger
	message string
	barrier chan struct{}
}

var (
	websocketLogQueue   = make(chan websocketLogEntry, websocketLogQueueSize)
	websocketLogDropped atomic.Uint64
)

func init() {
	go runWebsocketLogWriter()
}

func runWebsocketLogWriter() {
	for entry := range websocketLogQueue {
		if entry.barrier != nil {
			close(entry.barrier)
			continue
		}

		if entry.logger != nil {
			entry.logger.Info(entry.message)
		}
	}
}

// enqueueWebsocketLog keeps websocket request/response goroutines off the
// synchronous file I/O path. The queue is intentionally bounded: websocket
// ledger entries are best-effort diagnostics and must not create backpressure
// on application traffic when the log sink cannot keep up.
func enqueueWebsocketLog(log *zap.Logger, message string) bool {
	if log == nil {
		return true
	}

	select {
	case websocketLogQueue <- websocketLogEntry{logger: log, message: message}:
		return true
	default:
		websocketLogDropped.Add(1)
		return false
	}
}

// FlushWebsocketLogs waits until all websocket ledger entries queued before
// this call have reached their captured logger. It is primarily useful for
// tests and graceful shutdown paths.
func FlushWebsocketLogs() {
	barrier := make(chan struct{})
	websocketLogQueue <- websocketLogEntry{barrier: barrier}
	<-barrier
}

// DroppedWebsocketLogs reports how many ledger entries were discarded because
// the bounded async queue was full.
func DroppedWebsocketLogs() uint64 {
	return websocketLogDropped.Load()
}
