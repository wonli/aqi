package ws

import (
	"net"
	"testing"
	"time"
)

func TestUserLoginReplacesSameAppClientWithoutLosingNewClient(t *testing.T) {
	hub := NewHubc()
	oldConn, oldPeer := net.Pipe()
	defer oldPeer.Close()
	newConn, newPeer := net.Pipe()
	defer newPeer.Close()

	oldClient := &Client{Hub: hub, Conn: oldConn, Send: make(chan Message, 8)}
	newClient := &Client{Hub: hub, Conn: newConn, Send: make(chan Message, 8)}

	if err := hub.UserLogin("user-1", "app-1", oldClient); err != nil {
		t.Fatalf("first login failed: %v", err)
	}
	if got := hub.UserClient("user-1", "app-1"); got != oldClient {
		t.Fatalf("first login stored unexpected client: got %p want %p", got, oldClient)
	}

	if err := hub.UserLogin("user-1", "app-1", newClient); err != nil {
		t.Fatalf("replacement login failed: %v", err)
	}
	if got := hub.UserClient("user-1", "app-1"); got != newClient {
		t.Fatalf("replacement login stored unexpected client: got %p want %p", got, newClient)
	}

	select {
	case disconnected := <-hub.Disconnect:
		if disconnected != oldClient {
			t.Fatalf("replacement disconnected wrong client: got %p want %p", disconnected, oldClient)
		}
	default:
		t.Fatal("replacement login did not disconnect old client")
	}

	user := hub.User("user-1")
	if user == nil {
		t.Fatal("user missing after replacement login")
	}
	if got := user.ClientCount(); got != 1 {
		t.Fatalf("unexpected client count after replacement: got %d want 1", got)
	}

	// Simulate the stale disconnect event from the replaced connection. It must
	// not remove the replacement client that now owns the same appId.
	_, oldAppID, loggedIn := oldClient.LoginState()
	if !loggedIn {
		t.Fatal("old client should retain its login identity until disconnect cleanup")
	}
	if err := user.appLogout(oldAppID, oldClient); err != nil {
		t.Fatalf("stale logout failed: %v", err)
	}
	if got := hub.UserClient("user-1", "app-1"); got != newClient {
		t.Fatalf("stale logout removed replacement client: got %p want %p", got, newClient)
	}
	if got := user.ClientCount(); got != 1 {
		t.Fatalf("stale logout changed client count: got %d want 1", got)
	}
}

func TestUserLoginDifferentAppsShareSingleUser(t *testing.T) {
	hub := NewHubc()
	clientA := &Client{Hub: hub, Send: make(chan Message, 8)}
	clientB := &Client{Hub: hub, Send: make(chan Message, 8)}

	if err := hub.UserLogin("user-1", "app-a", clientA); err != nil {
		t.Fatalf("app-a login failed: %v", err)
	}
	if err := hub.UserLogin("user-1", "app-b", clientB); err != nil {
		t.Fatalf("app-b login failed: %v", err)
	}

	user := hub.User("user-1")
	if user == nil {
		t.Fatal("user missing after logins")
	}
	if got := user.ClientCount(); got != 2 {
		t.Fatalf("unexpected client count: got %d want 2", got)
	}
	if got := user.AppClient("app-a"); got != clientA {
		t.Fatalf("app-a lookup returned %p want %p", got, clientA)
	}
	if got := user.AppClient("app-b"); got != clientB {
		t.Fatalf("app-b lookup returned %p want %p", got, clientB)
	}

	userA, appA, loggedA := clientA.LoginState()
	userB, appB, loggedB := clientB.LoginState()
	if !loggedA || !loggedB || userA != user || userB != user || appA != "app-a" || appB != "app-b" {
		t.Fatalf("client login state mismatch: A=(%p,%q,%v) B=(%p,%q,%v)", userA, appA, loggedA, userB, appB, loggedB)
	}
}

func TestHubRunConnectionLoginReplacementAndDisconnectLifecycle(t *testing.T) {
	hub := NewHubc()
	go hub.Run()

	oldConn, oldPeer := net.Pipe()
	defer oldPeer.Close()
	newConn, newPeer := net.Pipe()
	defer newPeer.Close()

	oldClient := &Client{Hub: hub, Conn: oldConn, Send: make(chan Message, 8)}
	newClient := &Client{Hub: hub, Conn: newConn, Send: make(chan Message, 8)}

	hub.Connection <- oldClient
	waitFor(t, time.Second, func() bool {
		hub.clientsMu.RLock()
		_, ok := hub.Clients[oldClient]
		hub.clientsMu.RUnlock()
		return ok
	}, "old client was not registered by Hub.Run")

	if err := hub.UserLogin("user-1", "app-1", oldClient); err != nil {
		t.Fatalf("old client login failed: %v", err)
	}

	hub.Connection <- newClient
	waitFor(t, time.Second, func() bool {
		hub.clientsMu.RLock()
		_, ok := hub.Clients[newClient]
		hub.clientsMu.RUnlock()
		return ok
	}, "new client was not registered by Hub.Run")

	if err := hub.UserLogin("user-1", "app-1", newClient); err != nil {
		t.Fatalf("replacement login failed: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		hub.clientsMu.RLock()
		_, oldExists := hub.Clients[oldClient]
		_, newExists := hub.Clients[newClient]
		hub.clientsMu.RUnlock()
		return !oldExists && newExists
	}, "Hub.Run did not process the replaced client's disconnect")

	if got := hub.UserClient("user-1", "app-1"); got != newClient {
		t.Fatalf("replacement client lost after stale disconnect cleanup: got %p want %p", got, newClient)
	}
	user := hub.User("user-1")
	if user == nil || user.ClientCount() != 1 {
		t.Fatalf("unexpected user state after replacement: user=%p count=%d", user, clientCount(user))
	}

	newClient.Disconnect()
	waitFor(t, time.Second, func() bool {
		hub.clientsMu.RLock()
		_, exists := hub.Clients[newClient]
		hub.clientsMu.RUnlock()
		return !exists && user.ClientCount() == 0
	}, "Hub.Run did not complete final disconnect cleanup")
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(message)
}

func clientCount(user *User) int {
	if user == nil {
		return 0
	}
	return user.ClientCount()
}
