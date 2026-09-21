package ws

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

type capturingClusterTransport struct {
	*fakeClusterTransport
	payloadMu sync.Mutex
	payloads  map[string][][]byte
}

func newCapturingClusterTransport() *capturingClusterTransport {
	return &capturingClusterTransport{
		fakeClusterTransport: newFakeClusterTransport(),
		payloads:             make(map[string][][]byte),
	}
}

func (f *capturingClusterTransport) Publish(topic string, data []byte) error {
	f.payloadMu.Lock()
	f.payloads[topic] = append(f.payloads[topic], append([]byte(nil), data...))
	f.payloadMu.Unlock()
	return f.fakeClusterTransport.Publish(topic, data)
}

func (f *capturingClusterTransport) lastPayload(topic string) []byte {
	f.payloadMu.Lock()
	defer f.payloadMu.Unlock()
	items := f.payloads[topic]
	if len(items) == 0 {
		return nil
	}
	return append([]byte(nil), items[len(items)-1]...)
}

func installClusterDeliveryHub(t *testing.T) *Hubc {
	t.Helper()
	oldHub := Hub
	h := &Hubc{
		Clients: make(map[*Client]struct{}),
		Users:   new(sync.Map),
		PubSub:  NewPubSub(),
	}
	Hub = h
	t.Cleanup(func() { Hub = oldHub })
	return h
}

func addClusterDeliveryUser(h *Hubc, uid string) (*User, *Client) {
	client := &Client{Send: make(chan []byte, 8)}
	user := &User{
		Suid:       uid,
		Hub:        h,
		AppClients: []*Client{client},
		SubTopics:  make(map[string]*Topic),
	}
	client.setLoginState(user, "test")
	h.Users.Store(uid, user)
	return user, client
}

func remoteClusterWire(t *testing.T, data []byte) []byte {
	t.Helper()
	remote := [clusterNodeIDSize]byte{1}
	if remote == clusterCurrentNodeID() {
		remote[0] = 2
	}
	return clusterEncodeWire(remote, data)
}

func TestSendToUserDeliversLocallyAndPublishesForRemoteNodes(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newCapturingClusterTransport()
	setClusterTransport(transport)

	h := installClusterDeliveryHub(t)
	_, client := addClusterDeliveryUser(h, "B")

	if !h.SendToUser("B", []byte("hello")) {
		t.Fatal("SendToUser reported no delivery")
	}
	if got := <-client.Send; !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("local message = %q, want hello", got)
	}
	_, _, publishes := transport.counts("$aqi:user:B")
	if publishes != 1 {
		t.Fatalf("cluster publishes = %d, want 1", publishes)
	}
}

func TestClusterDeliveryDropsPublishingNodeSelfEcho(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newCapturingClusterTransport()
	setClusterTransport(transport)

	h := installClusterDeliveryHub(t)
	_, client := addClusterDeliveryUser(h, "B")

	h.SendToUser("B", []byte("hello"))
	<-client.Send

	wire := transport.lastPayload("$aqi:user:B")
	if len(wire) == 0 {
		t.Fatal("cluster publish did not capture a wire payload")
	}
	clusterHandleInbound("$aqi:user:B", wire)

	select {
	case got := <-client.Send:
		t.Fatalf("self echo delivered duplicate message %q", got)
	default:
	}
}

func TestClusterDeliveryRemoteUserDoesNotRepublish(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newCapturingClusterTransport()
	setClusterTransport(transport)

	h := installClusterDeliveryHub(t)
	_, client := addClusterDeliveryUser(h, "B")

	clusterHandleInbound("$aqi:user:B", remoteClusterWire(t, []byte("remote")))
	if got := <-client.Send; !bytes.Equal(got, []byte("remote")) {
		t.Fatalf("remote user message = %q, want remote", got)
	}
	_, _, publishes := transport.counts("$aqi:user:B")
	if publishes != 0 {
		t.Fatalf("inbound user delivery republished %d times", publishes)
	}
}

func TestPublishUsesSameEncodedPayloadLocallyAndRemotely(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newCapturingClusterTransport()
	setClusterTransport(transport)

	h := installClusterDeliveryHub(t)
	user, client := addClusterDeliveryUser(h, "B")
	topic := h.PubSub.initTopic("room:1")
	topic.AddSubUser(user)

	go h.PubSub.Start()
	t.Cleanup(func() { close(h.PubSub.TopicMsgQueue) })

	if !h.PubSub.Publish("room:1", H{"text": "hello"}) {
		t.Fatal("Publish reported no delivery")
	}

	var local []byte
	select {
	case local = <-client.Send:
	case <-time.After(time.Second):
		t.Fatal("local topic message was not delivered")
	}

	wire := transport.lastPayload("$aqi:topic:room:1")
	_, remoteData, ok := clusterDecodeWire(wire)
	if !ok {
		t.Fatal("cluster topic payload is not a valid wire message")
	}
	if !bytes.Equal(local, remoteData) {
		t.Fatalf("local and remote payloads differ\nlocal:  %q\nremote: %q", local, remoteData)
	}
}

func TestClusterDeliveryRemoteTopicSkipsHandlersAndRepublish(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newCapturingClusterTransport()
	setClusterTransport(transport)

	h := installClusterDeliveryHub(t)
	user, client := addClusterDeliveryUser(h, "B")
	topic := h.PubSub.initTopic("room:1")
	topic.AddSubUser(user)
	handlerCalls := 0
	topic.AddSubHandle(func(*TopicMsg) { handlerCalls++ })

	clusterHandleInbound("$aqi:topic:room:1", remoteClusterWire(t, []byte("encoded")))
	if got := <-client.Send; !bytes.Equal(got, []byte("encoded")) {
		t.Fatalf("remote topic message = %q, want encoded", got)
	}
	if handlerCalls != 0 {
		t.Fatalf("remote topic invoked %d process-local handlers, want 0", handlerCalls)
	}
	_, _, publishes := transport.counts("$aqi:topic:room:1")
	if publishes != 0 {
		t.Fatalf("inbound topic delivery republished %d times", publishes)
	}
}
