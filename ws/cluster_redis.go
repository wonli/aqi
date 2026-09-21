package ws

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
)

type redisClusterBackend interface {
	Subscribe(context.Context, string) error
	Unsubscribe(context.Context, string) error
	Publish(context.Context, string, []byte) error
	Receive(context.Context) (string, []byte, error)
	Close() error
}

type redisCluster struct {
	ctx       context.Context
	cancel    context.CancelFunc
	backend   redisClusterBackend
	onMessage func(string, []byte)

	startOnce sync.Once
	closeOnce sync.Once
	wg        sync.WaitGroup
	closeErr  error
}

func newRedisCluster(client *redis.Client, onMessage func(string, []byte)) (clusterTransport, error) {
	if client == nil {
		return nil, errors.New("aqi cluster: redis client is nil")
	}
	return newRedisClusterWithBackend(newGoRedisClusterBackend(client), onMessage), nil
}

func newRedisClusterWithBackend(backend redisClusterBackend, onMessage func(string, []byte)) *redisCluster {
	ctx, cancel := context.WithCancel(context.Background())
	return &redisCluster{
		ctx:       ctx,
		cancel:    cancel,
		backend:   backend,
		onMessage: onMessage,
	}
}

func (r *redisCluster) Subscribe(topic string) error {
	if err := r.backend.Subscribe(r.ctx, topic); err != nil {
		return err
	}
	r.startOnce.Do(func() {
		r.wg.Add(1)
		go r.receive()
	})
	return nil
}

func (r *redisCluster) Unsubscribe(topic string) error {
	return r.backend.Unsubscribe(r.ctx, topic)
}

func (r *redisCluster) Publish(topic string, data []byte) error {
	return r.backend.Publish(r.ctx, topic, data)
}

func (r *redisCluster) Close() error {
	r.closeOnce.Do(func() {
		r.cancel()
		r.closeErr = r.backend.Close()
		r.wg.Wait()
	})
	return r.closeErr
}

func (r *redisCluster) receive() {
	defer r.wg.Done()
	for {
		topic, data, err := r.backend.Receive(r.ctx)
		if err != nil {
			if r.ctx.Err() != nil {
				return
			}
			clusterLogError("receive", "redis", err)
			continue
		}
		if r.onMessage != nil {
			r.onMessage(topic, data)
		}
	}
}

type goRedisClusterBackend struct {
	client *redis.Client

	mu     sync.Mutex
	pubsub *redis.PubSub
	closed bool
}

func newGoRedisClusterBackend(client *redis.Client) *goRedisClusterBackend {
	return &goRedisClusterBackend{client: client}
}

func (b *goRedisClusterBackend) Subscribe(ctx context.Context, topic string) error {
	if topic == "" {
		return nil
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("aqi cluster: redis pubsub is closed")
	}
	if b.pubsub == nil {
		// Client.Subscribe keeps the subscription set even when the initial
		// network write fails; go-redis reconnects and resubscribes it later.
		b.pubsub = b.client.Subscribe(ctx, topic)
		b.mu.Unlock()
		return nil
	}
	pubsub := b.pubsub
	b.mu.Unlock()
	return pubsub.Subscribe(ctx, topic)
}

func (b *goRedisClusterBackend) Unsubscribe(ctx context.Context, topic string) error {
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
	return pubsub.Unsubscribe(ctx, topic)
}

func (b *goRedisClusterBackend) Publish(ctx context.Context, topic string, data []byte) error {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return errors.New("aqi cluster: redis pubsub is closed")
	}
	return b.client.Publish(ctx, topic, data).Err()
}

func (b *goRedisClusterBackend) Receive(ctx context.Context) (string, []byte, error) {
	b.mu.Lock()
	pubsub := b.pubsub
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return "", nil, errors.New("aqi cluster: redis pubsub is closed")
	}
	if pubsub == nil {
		return "", nil, errors.New("aqi cluster: redis pubsub is not subscribed")
	}

	msg, err := pubsub.ReceiveMessage(ctx)
	if err != nil {
		return "", nil, err
	}
	return msg.Channel, []byte(msg.Payload), nil
}

func (b *goRedisClusterBackend) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	pubsub := b.pubsub
	b.pubsub = nil
	b.mu.Unlock()

	if pubsub != nil {
		return pubsub.Close()
	}
	return nil
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
