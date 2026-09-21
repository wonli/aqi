package ws

import "testing"

func TestPubSubUnsubReportsOneMembershipTransition(t *testing.T) {
	pubsub := NewPubSub()
	user := &User{Suid: "B", SubTopics: make(map[string]*Topic)}

	pubsub.Sub("room:1", user)

	topicValue, ok := pubsub.Topics.Load("room:1")
	if !ok {
		t.Fatal("topic room:1 was not created")
	}
	topic := topicValue.(*Topic)

	if !pubsub.Unsub("room:1", user) {
		t.Fatal("first unsubscribe should report a membership transition")
	}
	if _, ok := user.SubTopics["room:1"]; ok {
		t.Fatal("user still contains room:1 after unsubscribe")
	}
	if _, ok := topic.SubUsers.Load(user.Suid); ok {
		t.Fatal("topic still contains user after unsubscribe")
	}

	if pubsub.Unsub("room:1", user) {
		t.Fatal("second unsubscribe should report no membership transition")
	}
}

func TestUserUnsubAllTopicsRemovesBothMembershipSides(t *testing.T) {
	pubsub := NewPubSub()
	user := &User{Suid: "B", SubTopics: make(map[string]*Topic)}

	pubsub.Sub("room:1", user)
	pubsub.Sub("room:2", user)

	if got := user.UnsubAllTopics(); got != 0 {
		t.Fatalf("remaining subscriptions = %d, want 0", got)
	}
	if len(user.SubTopics) != 0 {
		t.Fatalf("user subscriptions = %d, want 0", len(user.SubTopics))
	}

	for _, topicID := range []string{"room:1", "room:2"} {
		topicValue, ok := pubsub.Topics.Load(topicID)
		if !ok {
			t.Fatalf("topic %s was not created", topicID)
		}
		if _, ok := topicValue.(*Topic).SubUsers.Load(user.Suid); ok {
			t.Fatalf("topic %s still contains user after UnsubAllTopics", topicID)
		}
	}

	if got := user.UnsubAllTopics(); got != 0 {
		t.Fatalf("second UnsubAllTopics returned %d, want 0", got)
	}
}
