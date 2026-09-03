package ws

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/wonli/aqi/logger"
)

const (
	websocketLogQueueSize  = 8192
	websocketLogBatchSize  = 256
	websocketLogFlushEvery = 10 * time.Millisecond
)

type websocketLogEntry struct {
	message string
	barrier chan struct{}
}

var (
	websocketLogQueue   = make(chan websocketLogEntry, websocketLogQueueSize)
	websocketLogDropped atomic.Uint64
)

func init() {
	go runWebsocketLogBatchWriter()
}

func enqueueWebsocketLog(message string) bool {
	select {
	case websocketLogQueue <- websocketLogEntry{message: message}:
		return true
	default:
		websocketLogDropped.Add(1)
		return false
	}
}

func runWebsocketLogBatchWriter() {
	ticker := time.NewTicker(websocketLogFlushEvery)
	defer ticker.Stop()

	batch := make([]string, 0, websocketLogBatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if logger.FileLog != nil {
			logger.FileLog.Info(strings.Join(batch, "\n"))
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry := <-websocketLogQueue:
			if entry.barrier != nil {
				flush()
				close(entry.barrier)
				continue
			}

			batch = append(batch, entry.message)
			if len(batch) >= websocketLogBatchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// FlushWebsocketLogs waits until websocket ledger entries queued before this
// call have been written. It is useful for tests and graceful shutdown paths.
func FlushWebsocketLogs() {
	barrier := make(chan struct{})
	websocketLogQueue <- websocketLogEntry{barrier: barrier}
	<-barrier
}

// DroppedWebsocketLogs reports how many ledger entries were discarded because
// the bounded queue was full.
func DroppedWebsocketLogs() uint64 {
	return websocketLogDropped.Load()
}
