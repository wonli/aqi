package ws

import (
	"errors"
	"sync"
	"testing"
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
