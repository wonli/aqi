package ws

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wonli/aqi/logger"
)

const (
	clusterUserPrefix     = "$aqi:user:"
	clusterTopicPrefix    = "$aqi:topic:"
	clusterInstanceIDSize = 16
	clusterTopicLockCount = 64
)

var errClusterAlreadyInitialized = errors.New("aqi cluster: transport already initialized")

// ClusterTransport is the minimal transport contract AQI needs for inter-node routing.
// Channel names and payload bytes are opaque to the transport and must be preserved.
//
// Subscribe and Unsubscribe are idempotent desired-state operations. Before
// returning, including on error, an open transport must retain the latest desired
// membership for that channel. An error reports failure to apply it immediately,
// not rejection of the desired state. The transport must reconcile that state
// after recovery without requiring another AQI call. Newer intent must supersede
// older pending operations, so a delayed unsubscribe cannot undo a new subscribe.
// AQI may repeat calls, but does not schedule subscription retries and releases
// its local channel state after the last owner leaves, even if Unsubscribe fails.
// Implementations must support concurrent calls for different channels. Close
// ends reconciliation and releases subscriptions; calls after Close may fail.
type ClusterTransport interface {
	Subscribe(channel string) error
	Unsubscribe(channel string) error
	Publish(channel string, data []byte) error
	Close() error
}

// ClusterMessageHandler accepts one raw message received from the transport.
// Custom transports receive this handler from ClusterTransportFactory and call it
// with the exact channel and payload previously published by another AQI node.
type ClusterMessageHandler func(channel string, data []byte)

// ClusterTransportFactory builds a transport after AQI configuration has loaded.
// The supplied handler is the transport's inbound path back into AQI.
type ClusterTransportFactory func(ClusterMessageHandler) (ClusterTransport, error)

type clusterRuntime struct {
	transport  ClusterTransport
	instanceID [clusterInstanceIDSize]byte

	subscriptionsMu sync.Mutex
	subscriptions   map[string]*clusterSubscription
}

// Subscription fields are protected by the channel lock. The runtime mutex
// protects only the map shared by different channels.
type clusterSubscription struct {
	owners     map[*User]struct{}
	subscribed bool
}

var clusterState atomic.Pointer[clusterRuntime]
var clusterTopicLocks [clusterTopicLockCount]sync.Mutex

func clusterEnabled() bool {
	return clusterState.Load() != nil
}

func clusterTopicMutex(topic string) *sync.Mutex {
	var hash uint32 = 2166136261
	for i := 0; i < len(topic); i++ {
		hash ^= uint32(topic[i])
		hash *= 16777619
	}
	return &clusterTopicLocks[hash%clusterTopicLockCount]
}

func clusterUserChannel(uid string) string {
	return clusterUserPrefix + uid
}

func clusterTopicChannel(topicID string) string {
	return clusterTopicPrefix + topicID
}

func newClusterInstanceID() ([clusterInstanceIDSize]byte, error) {
	var id [clusterInstanceIDSize]byte
	_, err := rand.Read(id[:])
	return id, err
}

func installClusterTransport(t ClusterTransport, instanceID [clusterInstanceIDSize]byte) bool {
	return clusterState.CompareAndSwap(nil, &clusterRuntime{
		transport:     t,
		instanceID:    instanceID,
		subscriptions: make(map[string]*clusterSubscription),
	})
}

// InitClusterTransport creates and installs AQI's process-wide cluster transport.
// It is intended to be called once during AQI startup.
func InitClusterTransport(factory ClusterTransportFactory) error {
	if factory == nil {
		return errors.New("aqi cluster: transport factory is nil")
	}
	if clusterState.Load() != nil {
		return errClusterAlreadyInitialized
	}

	transport, err := factory(clusterHandleInbound)
	if err != nil {
		return err
	}
	if transport == nil {
		return errors.New("aqi cluster: transport factory returned nil transport")
	}

	instanceID, err := newClusterInstanceID()
	if err != nil {
		_ = transport.Close()
		return fmt.Errorf("aqi cluster: cannot generate instance id: %w", err)
	}
	if !installClusterTransport(transport, instanceID) {
		_ = transport.Close()
		return errClusterAlreadyInitialized
	}
	return nil
}

// clusterSyncUser reconciles one user's owned reference with current local state.
// The channel lock orders both reference changes and transport calls. Never call
// this while holding the user lock: inbound delivery also reads user state.
func clusterSyncUser(u *User, channel string) {
	c := clusterState.Load()
	if c == nil {
		return
	}
	topicMu := clusterTopicMutex(channel)
	topicMu.Lock()
	defer topicMu.Unlock()

	u.RLock()
	wanted := len(u.AppClients) > 0
	if channel != clusterUserChannel(u.Suid) {
		_, subscribed := u.SubTopics[strings.TrimPrefix(channel, clusterTopicPrefix)]
		wanted = wanted && subscribed
	}
	c.subscriptionsMu.Lock()
	subscription := c.subscriptions[channel]
	if subscription == nil && wanted {
		subscription = &clusterSubscription{owners: make(map[*User]struct{})}
		c.subscriptions[channel] = subscription
	}
	c.subscriptionsMu.Unlock()
	if subscription != nil {
		if wanted {
			subscription.owners[u] = struct{}{}
		} else {
			delete(subscription.owners, u)
		}
	}
	u.RUnlock()
	if subscription == nil {
		return
	}

	if len(subscription.owners) > 0 {
		if !subscription.subscribed {
			if err := c.transport.Subscribe(channel); err != nil {
				clusterLogError("subscribe", channel, err)
			} else {
				subscription.subscribed = true
			}
		}
		return
	}
	// The transport retains the unsubscribe intent even on error and owns recovery.
	// No owner remains, so AQI must release its bookkeeping regardless of delivery.
	if err := c.transport.Unsubscribe(channel); err != nil {
		clusterLogError("unsubscribe", channel, err)
	}
	c.subscriptionsMu.Lock()
	delete(c.subscriptions, channel)
	c.subscriptionsMu.Unlock()
}

func clusterPublish(topic string, data []byte) bool {
	c := clusterState.Load()
	if topic == "" || c == nil {
		return false
	}

	if err := c.transport.Publish(topic, clusterEncodeWire(c.instanceID, data)); err != nil {
		clusterLogError("publish", topic, err)
		return false
	}
	return true
}

func clusterEncodeWire(origin [clusterInstanceIDSize]byte, data []byte) []byte {
	wire := make([]byte, clusterInstanceIDSize+len(data))
	copy(wire[:clusterInstanceIDSize], origin[:])
	copy(wire[clusterInstanceIDSize:], data)
	return wire
}

func clusterDecodeWire(wire []byte) ([clusterInstanceIDSize]byte, []byte, bool) {
	var origin [clusterInstanceIDSize]byte
	if len(wire) < clusterInstanceIDSize {
		return origin, nil, false
	}
	copy(origin[:], wire[:clusterInstanceIDSize])
	return origin, wire[clusterInstanceIDSize:], true
}

func clusterHandleInbound(channel string, wire []byte) {
	c := clusterState.Load()
	if c == nil {
		return
	}

	origin, data, ok := clusterDecodeWire(wire)
	if !ok || origin == c.instanceID {
		return
	}

	h := Hub
	if h == nil {
		return
	}

	if uid, ok := strings.CutPrefix(channel, clusterUserPrefix); ok {
		if uid == "" {
			return
		}
		if user := h.User(uid); user != nil {
			user.SendMsg(data)
		}
		return
	}

	if topicID, ok := strings.CutPrefix(channel, clusterTopicPrefix); ok {
		if topicID == "" || h.PubSub == nil {
			return
		}
		h.PubSub.deliverCluster(topicID, data)
	}
}

func clusterLogError(operation, topic string, err error) {
	if logger.SugarLog != nil {
		logger.SugarLog.Errorf("cluster %s failed for %s: %v", operation, topic, err)
	}
}
