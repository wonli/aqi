//go:build ws_user_login_probe

package ws

import (
	"fmt"
	"sync"
	"testing"
)

func TestConcurrentFirstLoginKeepsSingleUserIdentity(t *testing.T) {
	const rounds = 200
	const clientsPerRound = 32

	hub := NewHubc()
	go hub.PubSub.Start()

	for round := 0; round < rounds; round++ {
		uid := fmt.Sprintf("concurrent-first-login-%d", round)
		clients := make([]*Client, clientsPerRound)
		start := make(chan struct{})

		var wg sync.WaitGroup
		for i := 0; i < clientsPerRound; i++ {
			client := &Client{Send: make(chan Message, 1)}
			clients[i] = client

			wg.Add(1)
			go func(index int, c *Client) {
				defer wg.Done()
				<-start
				if err := hub.UserLogin(uid, fmt.Sprintf("app-%d", index), c); err != nil {
					t.Errorf("round %d: UserLogin returned error: %v", round, err)
				}
			}(i, client)
		}

		close(start)
		wg.Wait()

		stored := hub.User(uid)
		if stored == nil {
			t.Fatalf("round %d: user missing from hub after login", round)
		}

		if got := stored.ClientCount(); got != clientsPerRound {
			t.Fatalf("round %d: stored user lost clients: got %d want %d", round, got, clientsPerRound)
		}

		for i, client := range clients {
			user, appID, loggedIn := client.LoginState()
			if !loggedIn {
				t.Fatalf("round %d client %d: client is not logged in", round, i)
			}
			if appID != fmt.Sprintf("app-%d", i) {
				t.Fatalf("round %d client %d: unexpected app id %q", round, i, appID)
			}
			if user != stored {
				t.Fatalf("round %d client %d: split user identity: client=%p hub=%p", round, i, user, stored)
			}
		}
	}
}
