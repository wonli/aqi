package ws

import (
	"sync"

	"github.com/wonli/aqi/logger"
)

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
}{
	refs: make(map[string]int),
}

func setClusterTransport(t clusterTransport) {
	clusterState.Lock()
	old := clusterState.transport
	clusterState.transport = t
	clusterState.refs = make(map[string]int)
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
	clusterState.Unlock()

	if old != nil {
		_ = old.Close()
	}
}

func clusterAcquire(topic string) {
	if topic == "" {
		return
	}

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
	clusterState.Unlock()
	if transport == nil {
		return false
	}

	if err := transport.Publish(topic, data); err != nil {
		clusterLogError("publish", topic, err)
		return false
	}
	return true
}

func clusterLogError(operation, topic string, err error) {
	if logger.SugarLog != nil {
		logger.SugarLog.Errorf("cluster %s failed for %s: %v", operation, topic, err)
	}
}
