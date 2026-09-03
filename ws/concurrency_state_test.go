package ws

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func newBufferedTestPubSub(size int) *PubSub {
	pubSub := NewPubSub()
	pubSub.TopicMsgQueue = make(chan *TopicMsg, size)
	return pubSub
}

func TestClientLoginStateConcurrentAccess(t *testing.T) {
	client := &Client{}
	users := []*User{{Suid: "u1"}, {Suid: "u2"}}

	const goroutines = 32
	const iterations = 2000

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				user := users[(id+j)%len(users)]
				client.setLoginState(user, fmt.Sprintf("app-%d", (id+j)%8))
			}
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				user, appID, loggedIn := client.LoginState()
				if loggedIn && (user == nil || appID == "") {
					t.Errorf("inconsistent login snapshot: user=%v appID=%q loggedIn=%v", user, appID, loggedIn)
					return
				}
				_ = client.IsLoggedIn()
			}
		}()
	}

	wg.Wait()
}

func TestClientAndUserActivityStateConcurrentAccess(t *testing.T) {
	user := &User{Suid: "activity-user"}
	client := &Client{}
	client.setLoginState(user, "app")

	const goroutines = 32
	const iterations = 2000

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				now := time.Unix(0, int64(offset*iterations+j+1))
				client.SetLastHeartbeat(now)
				client.TouchRequest(now)
			}
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = user.LastHeartbeat()
				_, _, _ = client.LoginState()
			}
		}()
	}

	wg.Wait()

	if user.LastHeartbeat().IsZero() {
		t.Fatal("expected heartbeat to be updated")
	}
}

func TestUserBanStateConcurrentAccess(t *testing.T) {
	user := &User{Suid: "ban-user"}

	const goroutines = 32
	const iterations = 2000

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if (id+j)%2 == 0 {
					user.Banned(time.Second)
				} else {
					user.Unban()
				}
			}
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, banTime := user.IsBanned()
				if banTime != nil {
					_ = banTime.UnixNano()
				}
			}
		}()
	}

	wg.Wait()
}

func TestUserAppClientsConcurrentAccess(t *testing.T) {
	const clientCount = 1000
	const readerGoroutines = 24

	hub := &Hubc{PubSub: newBufferedTestPubSub(clientCount*4 + 1024)}
	user := &User{
		Suid:       "registry-user",
		Hub:        hub,
		AppClients: make([]*Client, 0, clientCount),
		SubTopics:  make(map[string]*Topic),
	}

	clients := make([]*Client, clientCount)
	for i := range clients {
		clients[i] = &Client{Send: make(chan []byte, 1)}
	}

	stopReaders := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < readerGoroutines; i++ {
		readers.Add(1)
		go func(offset int) {
			defer readers.Done()
			index := offset
			for {
				select {
				case <-stopReaders:
					return
				default:
					appID := fmt.Sprintf("app-%d", index%clientCount)
					_ = user.ClientCount()
					_ = user.IsOnline()
					_ = user.AppClient(appID)
					user.SendMsg([]byte("race-check"))
					index++
				}
			}
		}(i)
	}

	var loginWG sync.WaitGroup
	for i, client := range clients {
		loginWG.Add(1)
		go func(index int, c *Client) {
			defer loginWG.Done()
			if err := user.appLogin(fmt.Sprintf("app-%d", index), c); err != nil {
				t.Errorf("appLogin returned error: %v", err)
			}
		}(i, client)
	}
	loginWG.Wait()

	if got := user.ClientCount(); got != clientCount {
		t.Fatalf("unexpected client count after login: got %d want %d", got, clientCount)
	}

	var logoutWG sync.WaitGroup
	for i, client := range clients {
		logoutWG.Add(1)
		go func(index int, c *Client) {
			defer logoutWG.Done()
			if err := user.appLogout(fmt.Sprintf("app-%d", index), c); err != nil {
				t.Errorf("appLogout returned error: %v", err)
			}
		}(i, client)
	}
	logoutWG.Wait()

	close(stopReaders)
	readers.Wait()

	if got := user.ClientCount(); got != 0 {
		t.Fatalf("unexpected client count after logout: got %d want 0", got)
	}
}

func TestHubBroadcastConcurrentRegistryMutation(t *testing.T) {
	const clientCount = 512
	const iterations = 2000

	hub := &Hubc{Clients: make(map[*Client]struct{}, clientCount)}
	clients := make([]*Client, clientCount)
	for i := range clients {
		clients[i] = &Client{Send: make(chan []byte, 1)}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			client := clients[i%clientCount]
			hub.clientsMu.Lock()
			if i%2 == 0 {
				hub.Clients[client] = struct{}{}
			} else {
				delete(hub.Clients, client)
			}
			hub.clientsMu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			hub.Broadcast([]byte("broadcast-race-check"))
		}
	}()

	wg.Wait()
}
