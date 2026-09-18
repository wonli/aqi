package ws

var maxFrameSize int64

// SetMaxFrameSize configures gobwas/ws Reader.MaxFrameSize for new WebSocket
// readers. Values <= 0 keep gobwas's unlimited behavior.
func SetMaxFrameSize(size int64) {
	maxFrameSize = size
}
