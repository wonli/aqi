package ws

import "github.com/gobwas/ws"

type Message struct {
	Op   ws.OpCode
	Data []byte
}
