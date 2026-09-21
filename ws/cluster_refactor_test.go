package ws

import (
	"sync"
	"testing"
	"time"
)

func TestDisabledClusterPublishKeepsLocalPubSemantics(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)

	pubsub := NewPubSub()
	preEncoded := make(chan bool, 1)
	pubsub.SubFunc("room:1", func(msg *TopicMsg) {
		preEncoded <- msg.Msg != nil
	})
	go pubsub.Start()
	t.Cleanup(func() { close(pubsub.TopicMsgQueue) })

	if !pubsub.Publish("room:1", H{"text": "hello"}) {
		t.Fatal("Publish reported no local delivery")
	}

	select {
	case got := <-preEncoded:
		if got {
			t.Fatal("disabled-cluster Publish encoded the message before local handlers ran")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for local topic handler")
	}
}

type loginBlockingTransport struct {
	*fakeClusterTransport
	started chan struct{}
	allow   chan struct{}
	once    sync.Once
}

func newLoginBlockingTransport() *loginBlockingTransport {
	return &loginBlockingTransport{
		fakeClusterTransport: newFakeClusterTransport(),
		started:              make(chan struct{}),
		allow:                make(chan struct{}),
	}
}

func (f *loginBlockingTransport) Subscribe(topic string) error {
	if topic == "$aqi:user:B" {
		f.once.Do(func() { close(f.started) })
		<-f.allow
	}
	return f.fakeClusterTransport.Subscribe(topic)
}

func TestReconnectConcurrentUnsubscribeDoesNotCreateGhostTopicRef(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)

	transport := newLoginBlockingTransport()
	setClusterTransport(transport)

	user := newClusterLifecycleUser("B")
	user.SubTopics["room:1"] = &Topic{Id: "room:1"}
	client := &Client{}

	loginDone := make(chan error, 1)
	go func() {
		loginDone <- user.appLogin("ios", client)
	}()

	select {
	case <-transport.started:
	case <-time.After(time.Second):
		t.Fatal("login did not reach cluster user subscribe")
	}

	user.UnsubTopic("room:1")
	close(transport.allow)

	select {
	case err := <-loginDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("login did not finish")
	}

	subs, _, _ := transport.counts("$aqi:topic:room:1")
	if subs != 0 {
		t.Fatalf("concurrent unsubscribe left %d ghost cluster topic subscription(s), want 0", subs)
	}
}
