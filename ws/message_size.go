package ws

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"
)

const DefaultMaxMessageSize int64 = 3 << 20

var (
	errMessageTooBig = errors.New("websocket message too big")
	messageSizeLimit atomic.Int64
)

func init() {
	messageSizeLimit.Store(DefaultMaxMessageSize)
}

// SetMaxMessageSize configures the maximum inbound WebSocket application
// message size. Configure it during startup before accepting connections.
func SetMaxMessageSize(size int64) error {
	if size <= 0 {
		return errors.New("websocket max message size must be greater than 0")
	}

	messageSizeLimit.Store(size)
	return nil
}

func maxMessageSize() int64 {
	return messageSizeLimit.Load()
}

func readMessagePayload(r io.Reader) ([]byte, error) {
	limit := maxMessageSize()
	payload, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, fmt.Errorf("%w: limit %d bytes", errMessageTooBig, limit)
	}

	return payload, nil
}
