package ws

import (
	"testing"
	"time"
)

func TestPubSubDoesNotEncodeInProcessPayloadWithoutUserSubscribers(t *testing.T) {
	pubSub := NewPubSub()
	received := make(chan *TopicMsg, 1)
	pubSub.SubFunc("connect", func(msg *TopicMsg) {
		received <- msg
	})

	client := &Client{Hub: &Hubc{Clients: map[*Client]struct{}{}}}
	if !pubSub.Pub("connect", client) {
		t.Fatal("publish failed")
	}

	go pubSub.Start()

	select {
	case msg := <-received:
		if msg.Ori != client {
			t.Fatal("subscriber did not receive original client")
		}
		if msg.Msg != nil {
			t.Fatal("in-process payload was encoded without websocket subscribers")
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive message")
	}
}
