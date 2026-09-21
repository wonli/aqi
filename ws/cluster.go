package ws

import (
	"bytes"
	"crypto/rand"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wonli/aqi/logger"
)

const clusterUserPrefix = "$aqi:user:"
const clusterTopicPrefix = "$aqi:topic:"
const clusterInstanceIDSize = 16
const clusterTopicLockCount = 64

type clusterTransport interface {
	Subscribe(topic string) error
	Unsubscribe(topic string) error
	Publish(topic string, data []byte) error
	Close() error
}

// clusterActive is fixed by cluster initialization during AQI startup in
// normal operation. It exists only as a lock-free fast path for single-node
// mode; transport/state ownership still lives in clusterState.
var clusterActive atomic.Bool

var clusterState = struct {
	sync.Mutex
	transport  clusterTransport
	refs       map[string]int
	instanceID [clusterInstanceIDSize]byte
}{
	refs: make(map[string]int),
}

// Subscription edge calls may involve network I/O. A small striped lock set
// keeps SUBSCRIBE/UNSUBSCRIBE ordered for the same topic without serializing
// unrelated topics behind one global network lock.
var clusterTopicLocks [clusterTopicLockCount]sync.Mutex

func clusterEnabled() bool {
	return clusterActive.Load()
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

func newClusterInstanceID() [clusterInstanceIDSize]byte {
	var id [clusterInstanceIDSize]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("aqi: cannot generate cluster instance id: " + err.Error())
	}
	return id
}

func setClusterTransport(t clusterTransport) {
	var instanceID [clusterInstanceIDSize]byte
	if t != nil {
		instanceID = newClusterInstanceID()
	}

	clusterState.Lock()
	old := clusterState.transport
	clusterState.transport = t
	clusterState.refs = make(map[string]int)
	clusterState.instanceID = instanceID
	clusterState.Unlock()

	clusterActive.Store(t != nil)

	if old != nil {
		_ = old.Close()
	}
}

func clearClusterTransport() {
	// Stop new hot-path operations before tearing down the transport.
	clusterActive.Store(false)

	clusterState.Lock()
	old := clusterState.transport
	clusterState.transport = nil
	clusterState.refs = make(map[string]int)
	clusterState.instanceID = [clusterInstanceIDSize]byte{}
	clusterState.Unlock()

	if old != nil {
		_ = old.Close()
	}
}

func clusterAcquire(topic string) {
	if topic == "" || !clusterEnabled() {
		return
	}

	topicMu := clusterTopicMutex(topic)
	topicMu.Lock()
	defer topicMu.Unlock()

	clusterState.Lock()
	transport := clusterState.transport
	if transport == nil {
		clusterState.Unlock()
		return
	}
	previous := clusterState.refs[topic]
	clusterState.refs[topic] = previous + 1
	clusterState.Unlock()

	if previous == 0 {
		if err := transport.Subscribe(topic); err != nil {
			clusterLogError("subscribe", topic, err)
		}
	}
}

func clusterRelease(topic string) {
	if topic == "" || !clusterEnabled() {
		return
	}

	topicMu := clusterTopicMutex(topic)
	topicMu.Lock()
	defer topicMu.Unlock()

	clusterState.Lock()
	transport := clusterState.transport
	if transport == nil {
		clusterState.Unlock()
		return
	}

	current := clusterState.refs[topic]
	if current <= 0 {
		clusterState.Unlock()
		return
	}

	current--
	if current == 0 {
		delete(clusterState.refs, topic)
	} else {
		clusterState.refs[topic] = current
	}
	clusterState.Unlock()

	if current == 0 {
		if err := transport.Unsubscribe(topic); err != nil {
			clusterLogError("unsubscribe", topic, err)
		}
	}
}

func clusterPublish(topic string, data []byte) bool {
	if topic == "" || !clusterEnabled() {
		return false
	}

	clusterState.Lock()
	transport := clusterState.transport
	instanceID := clusterState.instanceID
	clusterState.Unlock()
	if transport == nil {
		return false
	}

	if err := transport.Publish(topic, clusterEncodeWire(instanceID, data)); err != nil {
		clusterLogError("publish", topic, err)
		return false
	}
	return true
}

func clusterCurrentInstanceID() [clusterInstanceIDSize]byte {
	clusterState.Lock()
	defer clusterState.Unlock()
	return clusterState.instanceID
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
	return origin, append([]byte(nil), wire[clusterInstanceIDSize:]...), true
}

func clusterHandleInbound(channel string, wire []byte) {
	if !clusterEnabled() {
		return
	}

	origin, data, ok := clusterDecodeWire(wire)
	if !ok {
		return
	}

	clusterState.Lock()
	instanceID := clusterState.instanceID
	clusterState.Unlock()
	if bytes.Equal(origin[:], instanceID[:]) {
		return
	}

	h := Hub
	if h == nil {
		return
	}

	switch {
	case strings.HasPrefix(channel, clusterUserPrefix):
		uid := strings.TrimPrefix(channel, clusterUserPrefix)
		if uid == "" {
			return
		}
		if user := h.User(uid); user != nil {
			user.SendMsg(data)
		}
	case strings.HasPrefix(channel, clusterTopicPrefix):
		topicID := strings.TrimPrefix(channel, clusterTopicPrefix)
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
