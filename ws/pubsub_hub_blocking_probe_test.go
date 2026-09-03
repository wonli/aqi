//go:build ws_pubsub_hub_block_probe

package ws

import (
	"testing"
	"time"
)

func TestPubSubBackpressureDoesNotBlockHubLifecycle(t *testing.T) {
	hub := NewHubc()
	go hub.Run()

	blocked := make(chan struct{})
	release := make(chan struct{})
	hub.PubSub.SubFunc("slow-topic", func(msg *TopicMsg) {
		close(blocked)
		<-release
	})

	// Occupy the single PubSub consumer with a deliberately slow handler.
	hub.PubSub.Pub("slow-topic", "hold-consumer")
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("slow handler did not start")
	}

	// Fill the entire PubSub queue while its only consumer is blocked.
	for i := 0; i < cap(hub.PubSub.TopicMsgQueue); i++ {
		hub.PubSub.Pub("queued-topic", i)
	}

	client := &Client{Hub: hub, Send: make(chan Message, 1)}
	hub.Connection <- client

	// Hub.Run publishes the connect event synchronously. With a saturated
	// PubSub queue, it must still remain able to process lifecycle events.
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		hub.clientsMu.RLock()
		_, connected := hub.Clients[client]
		hub.clientsMu.RUnlock()
		if connected {
			break
		}
		time.Sleep(time.Millisecond)
	}

	client2 := &Client{Hub: hub, Send: make(chan Message, 1)}
	select {
	case hub.Connection <- client2:
	case <-time.After(300 * time.Millisecond):
		close(release)
		t.Fatal("hub lifecycle stalled: second connection could not be queued while PubSub was backpressured")
	}

	deadline = time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		hub.clientsMu.RLock()
		_, connected := hub.Clients[client2]
		hub.clientsMu.RUnlock()
		if connected {
			close(release)
			return
		}
		time.Sleep(time.Millisecond)
	}

	close(release)
	t.Fatal("hub lifecycle stalled behind synchronous PubSub publication")
}
