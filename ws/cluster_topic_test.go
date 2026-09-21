package ws

import "testing"

func newTopicTestUser(uid string, online bool) *User {
	user := &User{
		Suid:       uid,
		SubTopics:  make(map[string]*Topic),
		AppClients: []*Client{},
	}
	if online {
		user.AppClients = append(user.AppClients, &Client{})
	}
	return user
}

func TestClusterTopicUsesNodeLevelReferenceCounts(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	pubsub := NewPubSub()
	first := newTopicTestUser("A", true)
	second := newTopicTestUser("B", true)

	pubsub.Sub("room:1", first)
	pubsub.Sub("room:1", first)
	pubsub.Sub("room:1", second)

	subs, unsubs, _ := transport.counts("room:1")
	if subs != 1 || unsubs != 0 {
		t.Fatalf("after subscriptions = %d/%d, want 1/0", subs, unsubs)
	}

	if !pubsub.Unsub("room:1", first) {
		t.Fatal("first user's unsubscribe did not report a transition")
	}
	_, unsubs, _ = transport.counts("room:1")
	if unsubs != 0 {
		t.Fatalf("unsubscribe after one of two users = %d, want 0", unsubs)
	}

	if pubsub.Unsub("room:1", first) {
		t.Fatal("duplicate unsubscribe reported a transition")
	}
	_, unsubs, _ = transport.counts("room:1")
	if unsubs != 0 {
		t.Fatalf("duplicate unsubscribe changed transport count to %d", unsubs)
	}

	if !pubsub.Unsub("room:1", second) {
		t.Fatal("last user's unsubscribe did not report a transition")
	}
	_, unsubs, _ = transport.counts("room:1")
	if unsubs != 1 {
		t.Fatalf("last unsubscribe count = %d, want 1", unsubs)
	}
}

func TestClusterTopicOfflineRetainedUserDoesNotHoldRedisRef(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	pubsub := NewPubSub()
	user := newTopicTestUser("B", false)

	pubsub.Sub("room:1", user)
	subs, unsubs, _ := transport.counts("room:1")
	if subs != 0 || unsubs != 0 {
		t.Fatalf("offline subscription touched cluster = %d/%d, want 0/0", subs, unsubs)
	}
	if _, ok := user.SubTopics["room:1"]; !ok {
		t.Fatal("offline subscription was not retained locally")
	}
}

func TestClusterTopicDirectUserUnsubReleasesOnlineRef(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	pubsub := NewPubSub()
	user := newTopicTestUser("B", true)
	pubsub.Sub("room:1", user)

	if got := user.UnsubTopic("room:1"); got != 0 {
		t.Fatalf("remaining user topics = %d, want 0", got)
	}
	_, unsubs, _ := transport.counts("room:1")
	if unsubs != 1 {
		t.Fatalf("direct UnsubTopic unsubscribe count = %d, want 1", unsubs)
	}
}

func TestClusterTopicDirectUnsubAllReleasesOnlineRefs(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	pubsub := NewPubSub()
	user := newTopicTestUser("B", true)
	pubsub.Sub("room:1", user)
	pubsub.Sub("room:2", user)

	if got := user.UnsubAllTopics(); got != 0 {
		t.Fatalf("remaining user topics = %d, want 0", got)
	}
	for _, topic := range []string{"room:1", "room:2"} {
		_, unsubs, _ := transport.counts(topic)
		if unsubs != 1 {
			t.Fatalf("direct UnsubAllTopics %s unsubscribe count = %d, want 1", topic, unsubs)
		}
	}
}
