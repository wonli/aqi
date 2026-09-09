package ws

import "github.com/gobwas/ws"

type frame struct {
	op   ws.OpCode
	data []byte
}
