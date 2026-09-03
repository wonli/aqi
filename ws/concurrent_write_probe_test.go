//go:build ws_concurrent_write_probe

package ws

import (
	"bytes"
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/wonli/aqi/logger"
	"go.uber.org/zap"
)

// concurrentWriteProbeConn records whether multiple goroutines enter Write at
// the same time. The small delay widens the overlap window without changing
// Client production code.
type concurrentWriteProbeConn struct {
	net.Conn
	active     atomic.Int32
	overlapped atomic.Bool
}

func (c *concurrentWriteProbeConn) Write(p []byte) (int, error) {
	if c.active.Add(1) > 1 {
		c.overlapped.Store(true)
	}
	defer c.active.Add(-1)

	// Make concurrent outbound paths overlap deterministically enough for a
	// focused stress probe instead of relying on kernel timing alone.
	time.Sleep(5 * time.Millisecond)
	return c.Conn.Write(p)
}

func TestClientConcurrentConnectionWriteProbe(t *testing.T) {
	oldSugarLog := logger.SugarLog
	logger.SugarLog = zap.NewNop().Sugar()
	defer func() { logger.SugarLog = oldSugarLog }()

	serverConn, peerConn := net.Pipe()
	probeConn := &concurrentWriteProbeConn{Conn: serverConn}

	client := &Client{
		Conn:         probeConn,
		Send:         make(chan []byte, 128),
		RequestQueue: make(chan string, 1),
	}
	client.initContext(context.Background())

	readerDone := make(chan struct{})
	writerDone := make(chan struct{})
	peerReadDone := make(chan struct{})

	go func() {
		defer close(readerDone)
		client.Reader()
	}()
	go func() {
		defer close(writerDone)
		client.Write()
	}()

	// Drain server frames so net.Pipe writes can complete while the peer keeps
	// injecting Ping frames that make Reader write Pong directly.
	go func() {
		defer close(peerReadDone)
		for {
			if _, _, err := wsutil.ReadServerData(peerConn); err != nil {
				return
			}
		}
	}()

	payload := bytes.Repeat([]byte("x"), 32*1024)
	deadline := time.Now().Add(2 * time.Second)

	for i := 0; i < 200 && time.Now().Before(deadline) && !probeConn.overlapped.Load(); i++ {
		client.SendMsg(payload)

		// Client-side Ping is masked by wsutil.WriteClientMessage. Reader handles
		// it by calling WriteServerMessage(OpPong) on the same connection that the
		// Writer goroutine uses for text frames.
		if err := wsutil.WriteClientMessage(peerConn, ws.OpPing, nil); err != nil {
			break
		}
	}

	// Give an in-flight pair of writes one final scheduling window.
	for !probeConn.overlapped.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	_ = peerConn.Close()
	client.Disconnect()

	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("Reader did not exit")
	}
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("Writer did not exit")
	}
	select {
	case <-peerReadDone:
	case <-time.After(time.Second):
		t.Fatal("peer reader did not exit")
	}

	if probeConn.overlapped.Load() {
		t.Fatal("detected concurrent writes to the same WebSocket connection: Reader/Pong and Writer outbound path overlapped")
	}
}
