package ws

import (
	"bytes"
	"crypto/rand"
	"strings"
	"sync"

	"github.com/wonli/aqi/logger"
)

const clusterUserPrefix = "$user:"
const clusterNodeIDSize = 16
const clusterTopicLockCount = 64

type clusterTransport interface {
	Subscribe(topic string) error
	Unsubscribe(topic string) error
	Publish(topic string, data []byte) error
	Close() error
}

var clusterState = struct {
	sync.Mutex
	transport clusterTransport
	refs      map[string]int
	nodeID    [clusterNodeIDSize]byte
}{
	refs: make(map[string]int),
}

// Subscription edge calls may involve network I/O. A small striped lock set
// keeps SUBSCRIBE/UNSUBSCRIBE ordered for the same topic without serializing
// unrelated topics behind one global network lock.
var clusterTopicLocks [clusterTopicLockCount]sync.Mutex

func clusterTopicMutex(topic string) *sync.Mutex {
	var hash uint32 = 2166136261
	for i := 0; i < len(topic); i++ {
		hash ^= uint32(topic[i])
		hash *= 16777619
	}
	return &clusterTopicLocks[hash%clusterTopicLockCount]
}

func clusterUserTopic(uid string) string {
	return clusterUserPrefix + uid
}

func newClusterNodeID() [clusterNodeIDSize]byte {
	var id [clusterNodeIDSize]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("aqi: cannot generate cluster node id: " + err.Error())
	}
	return id
}

func setClusterTransport(t clusterTransport) {
	var nodeID [clusterNodeIDSize]byte
	if t != nil {
		nodeID = newClusterNodeID()
	}

	clusterState.Lock()
	old := clusterState.transport
	clusterState.transport = t
	clusterState.refs = make(map[string]int)
	clusterState.nodeID = nodeID
	clusterState.Unlock()

	if old != nil {
		_ = old.Close()
	}
}

func clearClusterTransport() {
	clusterState.Lock()
	old := clusterState.transport
	clusterState.transport = nil
	clusterState.refs = make(map[string]int)
	clusterState.nodeID = [clusterNodeIDSize]byte{}
	clusterState.Unlock()

	if old != nil {
		_ = old.Close()
	}
}

func clusterAcquire(topic string) {
	if topic == "" {
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
	if topic == "" {
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
	if topic == "" {
		return false
	}

	clusterState.Lock()
	transport := clusterState.transport
	nodeID := clusterState.nodeID
	clusterState.Unlock()
	if transport == nil {
		return false
	}

	if err := transport.Publish(topic, clusterEncodeWire(nodeID, data)); err != nil {
		clusterLogError("publish", topic, err)
		return false
	}
	return true
}

func clusterCurrentNodeID() [clusterNodeIDSize]byte {
	clusterState.Lock()
	defer clusterState.Unlock()
	return clusterState.nodeID
}

func clusterEncodeWire(origin [clusterNodeIDSize]byte, data []byte) []byte {
	wire := make([]byte, clusterNodeIDSize+len(data))
	copy(wire[:clusterNodeIDSize], origin[:])
	copy(wire[clusterNodeIDSize:], data)
	return wire
}

func clusterDecodeWire(wire []byte) ([clusterNodeIDSize]byte, []byte, bool) {
	var origin [clusterNodeIDSize]byte
	if len(wire) < clusterNodeIDSize {
		return origin, nil, false
	}
	copy(origin[:], wire[:clusterNodeIDSize])
	return origin, append([]byte(nil), wire[clusterNodeIDSize:]...), true
}

func clusterHandleInbound(topic string, wire []byte) {
	origin, data, ok := clusterDecodeWire(wire)
	if !ok {
		return
	}

	clusterState.Lock()
	enabled := clusterState.transport != nil
	nodeID := clusterState.nodeID
	clusterState.Unlock()
	if !enabled || bytes.Equal(origin[:], nodeID[:]) {
		return
	}

	h := Hub
	if h == nil {
		return
	}
	if strings.HasPrefix(topic, clusterUserPrefix) {
		uid := strings.TrimPrefix(topic, clusterUserPrefix)
		if uid == "" {
			return
		}
		if user := h.User(uid); user != nil {
			user.SendMsg(data)
		}
		return
	}
	if h.PubSub != nil {
		h.PubSub.deliverCluster(topic, data)
	}
}

func clusterLogError(operation, topic string, err error) {
	if logger.SugarLog != nil {
		logger.SugarLog.Errorf("cluster %s failed for %s: %v", operation, topic, err)
	}
}
