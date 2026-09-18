package ws

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/require"
)

func TestDefaultMaxMessageSizeIsThreeMiB(t *testing.T) {
	require.Equal(t, int64(3<<20), DefaultMaxMessageSize)
}

func TestReadMessagePayloadRespectsConfiguredLimit(t *testing.T) {
	old := maxMessageSize()
	require.NoError(t, SetMaxMessageSize(4))
	defer func() { require.NoError(t, SetMaxMessageSize(old)) }()

	payload, err := readMessagePayload(strings.NewReader("1234"))
	require.NoError(t, err)
	require.Equal(t, []byte("1234"), payload)

	payload, err = readMessagePayload(strings.NewReader("12345"))
	require.Nil(t, payload)
	require.ErrorIs(t, err, errMessageTooBig)
}

func TestSetMaxMessageSizeRejectsNonPositiveValues(t *testing.T) {
	old := maxMessageSize()
	require.Error(t, SetMaxMessageSize(0))
	require.Equal(t, old, maxMessageSize())
}

func TestReaderClosesOversizedMessageWith1009(t *testing.T) {
	old := maxMessageSize()
	require.NoError(t, SetMaxMessageSize(4))
	defer func() { require.NoError(t, SetMaxMessageSize(old)) }()

	serverConn, peerConn := net.Pipe()
	client := &Client{
		Conn:         serverConn,
		Send:         make(chan []byte, 1),
		RequestQueue: make(chan *Request, 1),
	}
	client.initContext(context.Background())

	readerDone := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		client.Reader()
	}()
	go func() {
		defer close(writerDone)
		client.Write()
	}()

	require.NoError(t, wsutil.WriteClientMessage(peerConn, ws.OpText, []byte("12345")))
	data, op, err := wsutil.ReadServerData(peerConn)
	require.NoError(t, err)
	require.Equal(t, ws.OpClose, op)
	code, reason := ws.ParseCloseFrameData(data)
	require.Equal(t, ws.StatusMessageTooBig, code)
	require.Equal(t, "message too big", reason)

	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("Reader did not exit after oversized message")
	}
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("Writer did not exit after close frame")
	}

	_ = peerConn.Close()
	if err := client.Context().Err(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected client context error: %v", err)
	}
}
