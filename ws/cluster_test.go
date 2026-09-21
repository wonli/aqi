package ws

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeClusterTransport struct {
	mu           sync.Mutex
	subscribes   map[string]int
	unsubscribes map[string]int
	publishes    map[string]int
	closed       int
	publishErr   error
}

func newFakeClusterTransport() *fakeClusterTransport {
	return &fakeClusterTransport{
		subscribes:   make(map[string]int),
		unsubscribes: make(map[string]int),
		publishes:    make(map[string]int),
	}
}

func (f *fakeClusterTransport) Subscribe(topic string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscribes[topic]++
	return nil
}

func (f *fakeClusterTransport) Unsubscribe(topic string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsubscribes[topic]++
	return nil
}

func (f *fakeClusterTransport) Publish(topic string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.publishes[topic]++
	return f.publishErr
}

func (f *fakeClusterTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return nil
}

func (f *fakeClusterTransport) counts(topic string) (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subscribes[topic], f.unsubscribes[topic], f.publishes[topic]
}

func TestClusterReferenceCountingUsesEdgeTransitions(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)

	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	clusterAcquire("room:1")
	clusterAcquire("room:1")

	subs, unsubs, _ := transport.counts("room:1")
	if subs != 1 || unsubs != 0 {
		t.Fatalf("after two acquires: subscribe=%d unsubscribe=%d, want 1/0", subs, unsubs)
	}

	clusterRelease("room:1")
	subs, unsubs, _ = transport.counts("room:1")
	if subs != 1 || unsubs != 0 {
		t.Fatalf("after first release: subscribe=%d unsubscribe=%d, want 1/0", subs, unsubs)
	}

	clusterRelease("room:1")
	subs, unsubs, _ = transport.counts("room:1")
	if subs != 1 || unsubs != 1 {
		t.Fatalf("after second release: subscribe=%d unsubscribe=%d, want 1/1", subs, unsubs)
	}

	clusterRelease("room:1")
	_, unsubs, _ = transport.counts("room:1")
	if unsubs != 1 {
		t.Fatalf("duplicate release unsubscribed %d times, want 1", unsubs)
	}
}

func TestClusterDisabledIsNoOp(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)

	clusterAcquire("room:1")
	clusterRelease("room:1")
	if clusterPublish("room:1", []byte("hello")) {
		t.Fatal("publish reported success with cluster disabled")
	}
}

func TestClusterDisabledFastPathDoesNotTouchTopicLock(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)

	topicMu := clusterTopicMutex("room:1")
	topicMu.Lock()
	defer topicMu.Unlock()

	done := make(chan struct{})
	go func() {
		clusterAcquire("room:1")
		clusterRelease("room:1")
		clusterPublish("room:1", []byte("hello"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("disabled cluster path blocked on topic lock")
	}
}

func TestClusterPublishReportsTransportFailure(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)

	transport := newFakeClusterTransport()
	transport.publishErr = errors.New("boom")
	setClusterTransport(transport)

	if clusterPublish("room:1", []byte("hello")) {
		t.Fatal("publish reported success when transport failed")
	}
	_, _, publishes := transport.counts("room:1")
	if publishes != 1 {
		t.Fatalf("publish calls = %d, want 1", publishes)
	}
}

type blockingClusterTransport struct {
	mu                sync.Mutex
	subscribeStarted  chan struct{}
	allowSubscribe    chan struct{}
	unsubscribeCalled chan struct{}
	startOnce         sync.Once
	unsubOnce         sync.Once
	subscribed        bool
}

func newBlockingClusterTransport() *blockingClusterTransport {
	return &blockingClusterTransport{
		subscribeStarted:  make(chan struct{}),
		allowSubscribe:    make(chan struct{}),
		unsubscribeCalled: make(chan struct{}),
	}
}

func (f *blockingClusterTransport) Subscribe(string) error {
	f.startOnce.Do(func() { close(f.subscribeStarted) })
	<-f.allowSubscribe
	f.mu.Lock()
	f.subscribed = true
	f.mu.Unlock()
	return nil
}

func (f *blockingClusterTransport) Unsubscribe(string) error {
	f.unsubOnce.Do(func() { close(f.unsubscribeCalled) })
	f.mu.Lock()
	f.subscribed = false
	f.mu.Unlock()
	return nil
}

func (f *blockingClusterTransport) Publish(string, []byte) error { return nil }
func (f *blockingClusterTransport) Close() error                 { return nil }

func (f *blockingClusterTransport) isSubscribed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subscribed
}

func TestClusterAcquireReleasePreservesFinalZeroSubscriptionState(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newBlockingClusterTransport()
	setClusterTransport(transport)

	acquireDone := make(chan struct{})
	go func() {
		clusterAcquire("room:1")
		close(acquireDone)
	}()
	<-transport.subscribeStarted

	releaseDone := make(chan struct{})
	go func() {
		clusterRelease("room:1")
		close(releaseDone)
	}()

	// A broken implementation can run UNSUBSCRIBE before the blocked SUBSCRIBE
	// finishes, leaving Redis subscribed even though the local ref count is zero.
	select {
	case <-transport.unsubscribeCalled:
	case <-time.After(20 * time.Millisecond):
	}
	close(transport.allowSubscribe)

	select {
	case <-acquireDone:
	case <-time.After(time.Second):
		t.Fatal("clusterAcquire did not finish")
	}
	select {
	case <-releaseDone:
	case <-time.After(time.Second):
		t.Fatal("clusterRelease did not finish")
	}

	if transport.isSubscribed() {
		t.Fatal("transport remained subscribed after acquire/release ended at zero refs")
	}
}
