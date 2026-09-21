package ws

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisClusterBackend struct {
	mu           sync.Mutex
	subscribed   map[string]bool
	subscribes   map[string]int
	unsubscribes map[string]int
	publishes    map[string]int
	messages     chan redisClusterMessage
	closed       bool
}

func newFakeRedisClusterBackend() *fakeRedisClusterBackend {
	return &fakeRedisClusterBackend{
		subscribed:   make(map[string]bool),
		subscribes:   make(map[string]int),
		unsubscribes: make(map[string]int),
		publishes:    make(map[string]int),
		messages:     make(chan redisClusterMessage, 16),
	}
}

func (f *fakeRedisClusterBackend) Subscribe(topic string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errors.New("closed")
	}
	f.subscribed[topic] = true
	f.subscribes[topic]++
	return nil
}

func (f *fakeRedisClusterBackend) Unsubscribe(topic string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errors.New("closed")
	}
	delete(f.subscribed, topic)
	f.unsubscribes[topic]++
	return nil
}

func (f *fakeRedisClusterBackend) Publish(topic string, data []byte) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return errors.New("closed")
	}
	f.publishes[topic]++
	subscribed := f.subscribed[topic]
	f.mu.Unlock()
	if subscribed {
		f.messages <- redisClusterMessage{topic: topic, data: append([]byte(nil), data...)}
	}
	return nil
}

func (f *fakeRedisClusterBackend) Messages() <-chan redisClusterMessage {
	return f.messages
}

func (f *fakeRedisClusterBackend) Close() error {
	f.mu.Lock()
	if !f.closed {
		f.closed = true
		close(f.messages)
	}
	f.mu.Unlock()
	return nil
}

func (f *fakeRedisClusterBackend) counts(topic string) (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subscribes[topic], f.unsubscribes[topic], f.publishes[topic]
}

func TestRedisClusterBackendFeedsInboundCallback(t *testing.T) {
	backend := newFakeRedisClusterBackend()
	messages := make(chan redisClusterMessage, 4)
	transport := newRedisClusterWithBackend(backend, func(topic string, data []byte) {
		messages <- redisClusterMessage{topic: topic, data: append([]byte(nil), data...)}
	})
	t.Cleanup(func() { _ = transport.Close() })

	if err := transport.Subscribe("room:1"); err != nil {
		t.Fatal(err)
	}
	if err := transport.Publish("room:1", []byte("one")); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-messages:
		if got.topic != "room:1" || !bytes.Equal(got.data, []byte("one")) {
			t.Fatalf("message = %s/%q, want room:1/one", got.topic, got.data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cluster backend message")
	}

	if err := transport.Subscribe("room:2"); err != nil {
		t.Fatal(err)
	}
	if err := transport.Publish("room:2", []byte("two")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-messages:
		if got.topic != "room:2" || !bytes.Equal(got.data, []byte("two")) {
			t.Fatalf("message = %s/%q, want room:2/two", got.topic, got.data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second cluster backend message")
	}

	if err := transport.Unsubscribe("room:1"); err != nil {
		t.Fatal(err)
	}
	if err := transport.Publish("room:1", []byte("ignored")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-messages:
		t.Fatalf("unsubscribed topic delivered %s/%q", got.topic, got.data)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestRedisClusterRuntimeCreatesOneBackendSubscriberPerTopic(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	backend := newFakeRedisClusterBackend()
	transport := newRedisClusterWithBackend(backend, clusterHandleInbound)
	setClusterTransport(transport)

	clusterAcquire("room:1")
	clusterAcquire("room:1")
	subs, unsubs, _ := backend.counts("room:1")
	if subs != 1 || unsubs != 0 {
		t.Fatalf("after two acquires subscribe/unsubscribe = %d/%d, want 1/0", subs, unsubs)
	}

	clusterRelease("room:1")
	_, unsubs, _ = backend.counts("room:1")
	if unsubs != 0 {
		t.Fatalf("first release unsubscribe = %d, want 0", unsubs)
	}
	clusterRelease("room:1")
	_, unsubs, _ = backend.counts("room:1")
	if unsubs != 1 {
		t.Fatalf("last release unsubscribe = %d, want 1", unsubs)
	}
}

func TestRedisClusterCloseClosesBackend(t *testing.T) {
	backend := newFakeRedisClusterBackend()
	transport := newRedisClusterWithBackend(backend, func(string, []byte) {})
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	closed := backend.closed
	backend.mu.Unlock()
	if !closed {
		t.Fatal("backend was not closed")
	}
}

func TestRedisClusterRejectsNilClient(t *testing.T) {
	if _, err := newRedisCluster(nil, func(string, []byte) {}); err == nil {
		t.Fatal("newRedisCluster accepted nil redis client")
	}
}

func TestGoRedisClusterFirstSubscribeKeepsIntentDuringOutage(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	backend := newGoRedisClusterBackend(client)
	t.Cleanup(func() { _ = backend.Close() })

	if err := backend.Subscribe("room:1"); err != nil {
		t.Fatalf("first subscribe should retain intent for go-redis reconnect, got %v", err)
	}

	backend.mu.Lock()
	pubsub := backend.pubsub
	backend.mu.Unlock()
	if pubsub == nil {
		t.Fatal("first subscribe did not retain the go-redis PubSub")
	}
}
