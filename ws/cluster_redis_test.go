package ws

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisReceiver struct {
	messages chan *redis.Message
	stopped  chan struct{}
	once     sync.Once
}

func newFakeRedisReceiver() *fakeRedisReceiver {
	return &fakeRedisReceiver{
		messages: make(chan *redis.Message, 4),
		stopped:  make(chan struct{}),
	}
}

func (f *fakeRedisReceiver) ReceiveMessage(ctx context.Context) (*redis.Message, error) {
	select {
	case msg := <-f.messages:
		return msg, nil
	case <-ctx.Done():
		f.once.Do(func() { close(f.stopped) })
		return nil, ctx.Err()
	}
}

func TestRedisClusterReceiveDispatchesInboundCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	messages := make(chan redis.Message, 1)
	transport := &redisCluster{
		ctx:    ctx,
		cancel: cancel,
		onMessage: func(topic string, data []byte) {
			messages <- redis.Message{Channel: topic, Payload: string(data)}
		},
	}
	receiver := newFakeRedisReceiver()
	transport.startReceiver(receiver)
	t.Cleanup(func() { _ = transport.Close() })

	receiver.messages <- &redis.Message{Channel: "room:1", Payload: "hello"}
	select {
	case got := <-messages:
		if got.Channel != "room:1" || !bytes.Equal([]byte(got.Payload), []byte("hello")) {
			t.Fatalf("message = %s/%q, want room:1/hello", got.Channel, got.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for redis cluster message")
	}
}

func TestRedisClusterCloseStopsReceiver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	transport := &redisCluster{ctx: ctx, cancel: cancel}
	receiver := newFakeRedisReceiver()
	transport.startReceiver(receiver)

	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-receiver.stopped:
	case <-time.After(time.Second):
		t.Fatal("redis receiver did not stop after Close")
	}
}

func TestRedisClusterRejectsNilClient(t *testing.T) {
	if _, err := newRedisCluster(nil, func(string, []byte) {}); err == nil {
		t.Fatal("newRedisCluster accepted nil redis client")
	}
}

func TestRedisClusterFirstSubscribeKeepsIntentDuringOutage(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	transport, err := newRedisCluster(client, func(string, []byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Close() })

	if err := transport.Subscribe("room:1"); err != nil {
		t.Fatalf("first subscribe should retain intent for go-redis reconnect, got %v", err)
	}

	transport.mu.Lock()
	pubsub := transport.pubsub
	transport.mu.Unlock()
	if pubsub == nil {
		t.Fatal("first subscribe did not retain the go-redis PubSub")
	}
}
