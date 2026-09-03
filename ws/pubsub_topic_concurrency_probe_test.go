//go:build ws_pubsub_topic_probe

package ws

import (
	"fmt"
	"sync"
	"testing"
)

func TestConcurrentTopicInitializationKeepsSingleTopicIdentity(t *testing.T) {
	const rounds = 200
	const subscribersPerRound = 32

	for round := 0; round < rounds; round++ {
		pubSub := NewPubSub()
		topicID := fmt.Sprintf("concurrent-topic-%d", round)
		users := make([]*User, subscribersPerRound)
		start := make(chan struct{})

		var wg sync.WaitGroup
		for i := 0; i < subscribersPerRound; i++ {
			user := &User{
				Suid:      fmt.Sprintf("user-%d", i),
				SubTopics: make(map[string]*Topic),
			}
			users[i] = user

			wg.Add(1)
			go func(u *User) {
				defer wg.Done()
				<-start
				pubSub.Sub(topicID, u)
			}(user)
		}

		close(start)
		wg.Wait()

		storedValue, ok := pubSub.Topics.Load(topicID)
		if !ok {
			t.Fatalf("round %d: topic missing after subscriptions", round)
		}
		stored := storedValue.(*Topic)

		subscriberCount := 0
		stored.SubUsers.Range(func(_, _ any) bool {
			subscriberCount++
			return true
		})
		if subscriberCount != subscribersPerRound {
			t.Fatalf("round %d: stored topic lost subscribers: got %d want %d", round, subscriberCount, subscribersPerRound)
		}

		for i, user := range users {
			user.RLock()
			topic := user.SubTopics[topicID]
			user.RUnlock()
			if topic != stored {
				t.Fatalf("round %d user %d: split topic identity: user=%p pubsub=%p", round, i, topic, stored)
			}
		}
	}
}
