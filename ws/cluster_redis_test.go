package ws

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newRedisClusterTestClient(t *testing.T) *redis.Client {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func waitRedisNumSub(t *testing.T, client *redis.Client, topic string, want int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		counts, err := client.PubSubNumSub(context.Background(), topic).Result()
		if err == nil && counts[topic] == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	counts, err := client.PubSubNumSub(context.Background(), topic).Result()
	t.Fatalf("subscriber count for %s = %v (err=%v), want %d", topic, counts, err, want)
}

func TestRedisClusterSubscribePublishAndUnsubscribe(t *testing.T) {
	client := newRedisClusterTestClient(t)
	messages := make(chan struct {
		topic string
		data  []byte
	}, 4)

	transport, err := newRedisCluster(client, func(topic string, data []byte) {
		messages <- struct {
			topic string
			data  []byte
		}{topic: topic, data: append([]byte(nil), data...)}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Close() })

	if err := transport.Subscribe("room:1"); err != nil {
		t.Fatal(err)
	}
	waitRedisNumSub(t, client, "room:1", 1)
	if err := transport.Publish("room:1", []byte("one")); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-messages:
		if got.topic != "room:1" || !bytes.Equal(got.data, []byte("one")) {
			t.Fatalf("message = %s/%q, want room:1/one", got.topic, got.data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for redis pubsub message")
	}

	if err := transport.Subscribe("room:2"); err != nil {
		t.Fatal(err)
	}
	waitRedisNumSub(t, client, "room:2", 1)
	if err := transport.Publish("room:2", []byte("two")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-messages:
		if got.topic != "room:2" || !bytes.Equal(got.data, []byte("two")) {
			t.Fatalf("message = %s/%q, want room:2/two", got.topic, got.data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second redis pubsub message")
	}

	if err := transport.Unsubscribe("room:1"); err != nil {
		t.Fatal(err)
	}
	waitRedisNumSub(t, client, "room:1", 0)
}

func TestRedisClusterRuntimeCreatesOneNodeSubscriberPerTopic(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	client := newRedisClusterTestClient(t)

	transport, err := newRedisCluster(client, clusterHandleInbound)
	if err != nil {
		t.Fatal(err)
	}
	setClusterTransport(transport)

	clusterAcquire("room:1")
	clusterAcquire("room:1")
	waitRedisNumSub(t, client, "room:1", 1)

	clusterRelease("room:1")
	waitRedisNumSub(t, client, "room:1", 1)
	clusterRelease("room:1")
	waitRedisNumSub(t, client, "room:1", 0)
}

func TestRedisClusterCloseStopsSubscriptions(t *testing.T) {
	client := newRedisClusterTestClient(t)
	transport, err := newRedisCluster(client, func(string, []byte) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := transport.Subscribe("room:1"); err != nil {
		t.Fatal(err)
	}
	waitRedisNumSub(t, client, "room:1", 1)
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	waitRedisNumSub(t, client, "room:1", 0)
}
