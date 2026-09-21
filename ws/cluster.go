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

	refsMu sync.Mutex
	refs   map[string]int
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
		transport:  t,
		instanceID: instanceID,
		refs:       make(map[string]int),
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

// setClusterTransport is a compact test helper for installing fake transports.
func setClusterTransport(t ClusterTransport) {
	clearClusterTransport()
	if t == nil {
		return
	}
	instanceID, err := newClusterInstanceID()
	if err != nil {
		panic("aqi: cannot generate cluster instance id: " + err.Error())
	}
	if !installClusterTransport(t, instanceID) {
		panic("aqi: cannot install test cluster transport")
	}
}

func clearClusterTransport() {
	if old := clusterState.Swap(nil); old != nil {
		_ = old.transport.Close()
	}
}

func clusterAcquire(topic string) {
	c := clusterState.Load()
	if topic == "" || c == nil {
		return
	}

	topicMu := clusterTopicMutex(topic)
	topicMu.Lock()
	defer topicMu.Unlock()

	c.refsMu.Lock()
	previous := c.refs[topic]
	c.refs[topic] = previous + 1
	c.refsMu.Unlock()

	if previous == 0 {
		if err := c.transport.Subscribe(topic); err != nil {
			clusterLogError("subscribe", topic, err)
		}
	}
}

func clusterRelease(topic string) {
	c := clusterState.Load()
	if topic == "" || c == nil {
		return
	}

	topicMu := clusterTopicMutex(topic)
	topicMu.Lock()
	defer topicMu.Unlock()

	c.refsMu.Lock()
	current := c.refs[topic]
	if current <= 0 {
		c.refsMu.Unlock()
		return
	}

	current--
	if current == 0 {
		delete(c.refs, topic)
	} else {
		c.refs[topic] = current
	}
	c.refsMu.Unlock()

	if current == 0 {
		if err := c.transport.Unsubscribe(topic); err != nil {
			clusterLogError("unsubscribe", topic, err)
		}
	}
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

func clusterCurrentInstanceID() [clusterInstanceIDSize]byte {
	if c := clusterState.Load(); c != nil {
		return c.instanceID
	}
	return [clusterInstanceIDSize]byte{}
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
