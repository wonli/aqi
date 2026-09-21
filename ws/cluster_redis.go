package ws

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
)

type redisClusterMessage struct {
	topic string
	data  []byte
}

type redisClusterBackend interface {
	Subscribe(topic string) error
	Unsubscribe(topic string) error
	Publish(topic string, data []byte) error
	Messages() <-chan redisClusterMessage
	Close() error
}

type redisCluster struct {
	backend   redisClusterBackend
	onMessage func(topic string, data []byte)
	closeOnce sync.Once
	wg        sync.WaitGroup
	closeErr  error
}

func newRedisCluster(client *redis.Client, onMessage func(topic string, data []byte)) (clusterTransport, error) {
	if client == nil {
		return nil, errors.New("aqi cluster: redis client is nil")
	}
	return newRedisClusterWithBackend(newGoRedisClusterBackend(client), onMessage), nil
}

func newRedisClusterWithBackend(backend redisClusterBackend, onMessage func(topic string, data []byte)) clusterTransport {
	r := &redisCluster{backend: backend, onMessage: onMessage}
	r.wg.Add(1)
	go r.receive()
	return r
}

func (r *redisCluster) Subscribe(topic string) error {
	return r.backend.Subscribe(topic)
}

func (r *redisCluster) Unsubscribe(topic string) error {
	return r.backend.Unsubscribe(topic)
}

func (r *redisCluster) Publish(topic string, data []byte) error {
	return r.backend.Publish(topic, data)
}

func (r *redisCluster) Close() error {
	r.closeOnce.Do(func() {
		r.closeErr = r.backend.Close()
		r.wg.Wait()
	})
	return r.closeErr
}

func (r *redisCluster) receive() {
	defer r.wg.Done()
	for msg := range r.backend.Messages() {
		if r.onMessage != nil {
			r.onMessage(msg.topic, append([]byte(nil), msg.data...))
		}
	}
}

type goRedisClusterBackend struct {
	client *redis.Client
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	pubsub   *redis.PubSub
	messages chan redisClusterMessage
	closed   bool
	wg       sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

func newGoRedisClusterBackend(client *redis.Client) *goRedisClusterBackend {
	ctx, cancel := context.WithCancel(context.Background())
	return &goRedisClusterBackend{
		client:   client,
		ctx:      ctx,
		cancel:   cancel,
		messages: make(chan redisClusterMessage, 128),
	}
}

func (b *goRedisClusterBackend) Subscribe(topic string) error {
	if topic == "" {
		return nil
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("aqi cluster: redis pubsub is closed")
	}
	if b.pubsub == nil {
		pubsub := b.client.Subscribe(b.ctx, topic)
		if _, err := pubsub.Receive(b.ctx); err != nil {
			_ = pubsub.Close()
			b.mu.Unlock()
			return err
		}
		b.pubsub = pubsub
		b.wg.Add(1)
		go b.receive(pubsub)
		b.mu.Unlock()
		return nil
	}
	pubsub := b.pubsub
	b.mu.Unlock()
	return pubsub.Subscribe(b.ctx, topic)
}

func (b *goRedisClusterBackend) Unsubscribe(topic string) error {
	if topic == "" {
		return nil
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("aqi cluster: redis pubsub is closed")
	}
	pubsub := b.pubsub
	b.mu.Unlock()
	if pubsub == nil {
		return nil
	}
	return pubsub.Unsubscribe(b.ctx, topic)
}

func (b *goRedisClusterBackend) Publish(topic string, data []byte) error {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return errors.New("aqi cluster: redis pubsub is closed")
	}
	return b.client.Publish(b.ctx, topic, data).Err()
}

func (b *goRedisClusterBackend) Messages() <-chan redisClusterMessage {
	return b.messages
}

func (b *goRedisClusterBackend) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		pubsub := b.pubsub
		b.pubsub = nil
		b.mu.Unlock()

		b.cancel()
		if pubsub != nil {
			b.closeErr = pubsub.Close()
		}
		b.wg.Wait()
		close(b.messages)
	})
	return b.closeErr
}

func (b *goRedisClusterBackend) receive(pubsub *redis.PubSub) {
	defer b.wg.Done()
	for {
		msg, err := pubsub.ReceiveMessage(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			clusterLogError("receive", "redis", err)
			continue
		}

		select {
		case b.messages <- redisClusterMessage{topic: msg.Channel, data: []byte(msg.Payload)}:
		case <-b.ctx.Done():
			return
		}
	}
}

// InitCluster installs Redis Pub/Sub as AQI's realtime inter-node transport.
func InitCluster(client *redis.Client) error {
	transport, err := newRedisCluster(client, clusterHandleInbound)
	if err != nil {
		return err
	}
	setClusterTransport(transport)
	return nil
}

// CloseCluster stops the current inter-node transport if one is enabled.
func CloseCluster() {
	clearClusterTransport()
}
