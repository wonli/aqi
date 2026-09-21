package ws

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
)

type redisReceiver interface {
	ReceiveMessage(context.Context) (*redis.Message, error)
}

type redisCluster struct {
	client    *redis.Client
	ctx       context.Context
	cancel    context.CancelFunc
	onMessage func(string, []byte)

	mu        sync.Mutex
	pubsub    *redis.PubSub
	startOnce sync.Once
	closeOnce sync.Once
	wg        sync.WaitGroup
	closeErr  error
}

func newRedisCluster(client *redis.Client, onMessage func(string, []byte)) (*redisCluster, error) {
	if client == nil {
		return nil, errors.New("aqi cluster: redis client is nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &redisCluster{
		client:    client,
		ctx:       ctx,
		cancel:    cancel,
		onMessage: onMessage,
	}, nil
}

func (r *redisCluster) Subscribe(topic string) error {
	if topic == "" {
		return nil
	}
	if err := r.ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	if r.pubsub == nil {
		// Client.Subscribe keeps the subscription set even when the initial
		// network write fails; go-redis reconnects and resubscribes it later.
		r.pubsub = r.client.Subscribe(r.ctx, topic)
		r.startReceiver(r.pubsub)
		r.mu.Unlock()
		return nil
	}
	pubsub := r.pubsub
	r.mu.Unlock()
	return pubsub.Subscribe(r.ctx, topic)
}

func (r *redisCluster) Unsubscribe(topic string) error {
	if topic == "" {
		return nil
	}
	if err := r.ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	pubsub := r.pubsub
	r.mu.Unlock()
	if pubsub == nil {
		return nil
	}
	return pubsub.Unsubscribe(r.ctx, topic)
}

func (r *redisCluster) Publish(topic string, data []byte) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	return r.client.Publish(r.ctx, topic, data).Err()
}

func (r *redisCluster) Close() error {
	r.closeOnce.Do(func() {
		r.cancel()

		r.mu.Lock()
		pubsub := r.pubsub
		r.pubsub = nil
		r.mu.Unlock()

		if pubsub != nil {
			r.closeErr = pubsub.Close()
		}
		r.wg.Wait()
	})
	return r.closeErr
}

func (r *redisCluster) startReceiver(receiver redisReceiver) {
	r.startOnce.Do(func() {
		r.wg.Add(1)
		go r.receive(receiver)
	})
}

func (r *redisCluster) receive(receiver redisReceiver) {
	defer r.wg.Done()
	for {
		msg, err := receiver.ReceiveMessage(r.ctx)
		if err != nil {
			if r.ctx.Err() != nil {
				return
			}
			clusterLogError("receive", "redis", err)
			continue
		}
		if r.onMessage != nil {
			r.onMessage(msg.Channel, []byte(msg.Payload))
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
