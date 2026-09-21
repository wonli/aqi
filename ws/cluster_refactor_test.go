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

	subs, unsubs, _ := transport.counts("$aqi:topic:room:1")
	if subs != unsubs {
		t.Fatalf("concurrent unsubscribe left unbalanced cluster topic refs: subscribe=%d unsubscribe=%d", subs, unsubs)
	}
}

func TestSubscribeConcurrentLoginDoesNotDoubleAcquireTopicRef(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)

	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	pubsub := NewPubSub()
	user := newClusterLifecycleUser("B")
	client := &Client{}

	// Hold the User lock until Sub has already added the Topic-side membership.
	// Sub is then blocked adding User.SubTopics; appLogin queues behind it. When
	// the lock opens, the current split "add topic, then IsOnline" path can let
	// login snapshot the new topic before Sub checks online and acquire it twice.
	user.Lock()
	subDone := make(chan struct{})
	go func() {
		pubsub.Sub("room:1", user)
		close(subDone)
	}()

	var topic *Topic
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if value, ok := pubsub.Topics.Load("room:1"); ok {
			topic = value.(*Topic)
			if _, ok := topic.SubUsers.Load(user.Suid); ok {
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if topic == nil {
		user.Unlock()
		t.Fatal("subscribe did not create topic membership")
	}

	loginDone := make(chan error, 1)
	go func() {
		loginDone <- user.appLogin("ios", client)
	}()
	// Give appLogin time to queue its writer behind the blocked Sub writer.
	time.Sleep(10 * time.Millisecond)
	user.Unlock()

	select {
	case <-subDone:
	case <-time.After(time.Second):
		t.Fatal("subscribe did not finish")
	}
	select {
	case err := <-loginDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("login did not finish")
	}

	user.UnsubTopic("room:1")
	subs, unsubs, _ := transport.counts("$aqi:topic:room:1")
	if subs != unsubs {
		t.Fatalf("subscribe/login race left unbalanced topic refs: subscribe=%d unsubscribe=%d", subs, unsubs)
	}
}
