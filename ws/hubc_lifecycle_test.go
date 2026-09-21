package ws

import (
	"testing"
	"time"
)

func TestUserLoginAndCleanupShareLifecycleLock(t *testing.T) {
	h := NewHubc()
	user := NewUser("B")
	user.SetLastHeartbeat(time.Now().Add(-10 * time.Minute))
	h.Users.Store("B", user)

	lifecycleMu := h.userLifecycleMutex("B")
	lifecycleMu.Lock()

	cleanupDone := make(chan bool, 1)
	go func() {
		cleanupDone <- h.cleanupUser("B", user, time.Now().Add(-5*time.Minute))
	}()

	loginDone := make(chan error, 1)
	go func() {
		loginDone <- h.UserLogin("B", "ios", &Client{})
	}()

	select {
	case <-cleanupDone:
		t.Fatal("cleanup bypassed user lifecycle lock")
	case <-loginDone:
		t.Fatal("login bypassed user lifecycle lock")
	case <-time.After(20 * time.Millisecond):
	}

	lifecycleMu.Unlock()

	select {
	case <-cleanupDone:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not finish")
	}
	select {
	case err := <-loginDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("login did not finish")
	}

	current := h.User("B")
	if current == nil || current.ClientCount() != 1 {
		t.Fatalf("reconnected user was lost during cleanup: user=%v clients=%d", current != nil, func() int {
			if current == nil {
				return 0
			}
			return current.ClientCount()
		}())
	}
}
