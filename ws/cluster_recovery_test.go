package ws

import (
	"errors"
	"testing"
)

type recoveringClusterTransport struct {
	*fakeClusterTransport
	unavailable bool
	active      map[string]bool
	desired     map[string]bool
}

func (r *recoveringClusterTransport) Subscribe(channel string) error {
	if r.desired == nil {
		r.desired = make(map[string]bool)
	}
	r.desired[channel] = true
	if r.unavailable {
		return errors.New("temporarily unavailable")
	}
	r.active[channel] = true
	return nil
}

func (r *recoveringClusterTransport) Unsubscribe(channel string) error {
	delete(r.desired, channel)
	if r.unavailable {
		return errors.New("temporarily unavailable")
	}
	delete(r.active, channel)
	return nil
}

// recover models a backend reconnect: it applies the latest retained intent
// without calling AQI or replaying stale operations.
func (r *recoveringClusterTransport) recover() {
	r.unavailable = false
	clear(r.active)
	for channel := range r.desired {
		r.active[channel] = true
	}
}

func TestClusterSubscriptionRecoversOnNextSubscribe(t *testing.T) {
	for _, repeat := range []bool{true, false} {
		name := "new user"
		if repeat {
			name = "same user"
		}
		t.Run(name, func(t *testing.T) {
			clearClusterTransport()
			t.Cleanup(clearClusterTransport)
			transport := &recoveringClusterTransport{fakeClusterTransport: newFakeClusterTransport(), unavailable: true, active: make(map[string]bool)}
			setClusterTransport(transport)
			pubsub := NewPubSub()
			first := newTopicTestUser("A", true)
			pubsub.Sub("room:1", first)
			transport.unavailable = false
			second := first
			if !repeat {
				second = newTopicTestUser("B", true)
			}
			pubsub.Sub("room:1", second)
			if !transport.active["$aqi:topic:room:1"] {
				t.Fatal("recovered transport did not subscribe")
			}
			pubsub.Unsub("room:1", first)
			if !repeat && !transport.active["$aqi:topic:room:1"] {
				t.Fatal("removed another user's subscription")
			}
			pubsub.Unsub("room:1", second)
			if transport.active["$aqi:topic:room:1"] {
				t.Fatal("last unsubscribe left transport active")
			}
		})
	}
}

func TestTopicAddSubUserSynchronizesCluster(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)
	topic := NewPubSub().initTopic("room:1")
	user := newTopicTestUser("A", true)
	topic.AddSubUser(user)
	topic.AddSubUser(user)
	subs, _, _ := transport.counts("$aqi:topic:room:1")
	if subs != 1 {
		t.Fatalf("Subscribe calls = %d, want 1", subs)
	}
	user.UnsubTopic("room:1")
	_, unsubs, _ := transport.counts("$aqi:topic:room:1")
	if unsubs != 1 {
		t.Fatalf("Unsubscribe calls = %d, want 1", unsubs)
	}
}

func TestClusterFailedSubscribeIntentIsCancelled(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := &recoveringClusterTransport{fakeClusterTransport: newFakeClusterTransport(), unavailable: true, active: make(map[string]bool)}
	setClusterTransport(transport)
	pubsub := NewPubSub()
	user := newTopicTestUser("A", true)
	pubsub.Sub("room:1", user)
	// A transport can retain and later activate intent even after returning an error.
	transport.active["$aqi:topic:room:1"] = true
	transport.unavailable = false
	pubsub.Unsub("room:1", user)
	if transport.active["$aqi:topic:room:1"] {
		t.Fatal("failed subscribe intent remained active after last unsubscribe")
	}
}

func TestClusterResubscribeAfterFailedUnsubscribe(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := &recoveringClusterTransport{fakeClusterTransport: newFakeClusterTransport(), active: make(map[string]bool)}
	setClusterTransport(transport)
	pubsub := NewPubSub()
	user := newTopicTestUser("A", true)
	pubsub.Sub("room:1", user)
	transport.unavailable = true
	pubsub.Unsub("room:1", user)
	// A failed operation can still have reached the backend before the error.
	delete(transport.active, "$aqi:topic:room:1")
	transport.unavailable = false
	pubsub.Sub("room:1", user)
	if !transport.active["$aqi:topic:room:1"] {
		t.Fatal("failed unsubscribe prevented resubscription")
	}
}

func TestClusterFailedUnsubscribeReleasesLocalStateWithoutFurtherActivity(t *testing.T) {
	for _, userChannel := range []bool{false, true} {
		name := "topic"
		if userChannel {
			name = "user"
		}
		t.Run(name, func(t *testing.T) {
			clearClusterTransport()
			t.Cleanup(clearClusterTransport)
			transport := &recoveringClusterTransport{fakeClusterTransport: newFakeClusterTransport(), active: make(map[string]bool)}
			setClusterTransport(transport)
			user := newClusterLifecycleUser("B")
			client := &Client{}
			channel := "$aqi:topic:room:1"
			if userChannel {
				channel = "$aqi:user:B"
				if err := user.appLogin("ios", client); err != nil {
					t.Fatal(err)
				}
			} else {
				user.AppClients = []*Client{client}
				user.Hub.PubSub.Sub("room:1", user)
			}
			transport.unavailable = true
			if userChannel {
				if err := user.appLogout("ios", client); err != nil {
					t.Fatal(err)
				}
			} else {
				user.UnsubTopic("room:1")
			}
			state := clusterState.Load()
			state.subscriptionsMu.Lock()
			_, retained := state.subscriptions[channel]
			state.subscriptionsMu.Unlock()
			if retained {
				t.Fatal("failed unsubscribe retained ownerless local state")
			}
			transport.recover()
			if transport.active[channel] {
				t.Fatal("transport restored a cancelled subscription after recovery")
			}
		})
	}
}
