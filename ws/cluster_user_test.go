package ws

import (
	"net"
	"testing"
)

func newClusterLifecycleUser(uid string) *User {
	return &User{
		Suid:       uid,
		Hub:        &Hubc{PubSub: NewPubSub()},
		AppClients: []*Client{},
		SubTopics:  make(map[string]*Topic),
	}
}

func TestClusterUserTracksOnlyOnlineEdges(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	user := newClusterLifecycleUser("B")
	first := &Client{}
	second := &Client{}

	if err := user.appLogin("iphone", first); err != nil {
		t.Fatal(err)
	}
	subs, unsubs, _ := transport.counts("$user:B")
	if subs != 1 || unsubs != 0 {
		t.Fatalf("first login subscribe/unsubscribe = %d/%d, want 1/0", subs, unsubs)
	}

	if err := user.appLogin("ipad", second); err != nil {
		t.Fatal(err)
	}
	subs, unsubs, _ = transport.counts("$user:B")
	if subs != 1 || unsubs != 0 {
		t.Fatalf("second login subscribe/unsubscribe = %d/%d, want 1/0", subs, unsubs)
	}

	if err := user.appLogout("iphone", first); err != nil {
		t.Fatal(err)
	}
	_, unsubs, _ = transport.counts("$user:B")
	if unsubs != 0 {
		t.Fatalf("first logout unsubscribe = %d, want 0", unsubs)
	}

	if err := user.appLogout("ipad", second); err != nil {
		t.Fatal(err)
	}
	_, unsubs, _ = transport.counts("$user:B")
	if unsubs != 1 {
		t.Fatalf("last logout unsubscribe = %d, want 1", unsubs)
	}
}

func TestSameAppClusterReplacementDoesNotGoOffline(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	user := newClusterLifecycleUser("B")
	oldConn, oldPeer := net.Pipe()
	newConn, newPeer := net.Pipe()
	defer oldPeer.Close()
	defer newConn.Close()
	defer newPeer.Close()

	oldClient := &Client{Conn: oldConn}
	newClient := &Client{Conn: newConn}

	if err := user.appLogin("ios", oldClient); err != nil {
		t.Fatal(err)
	}
	if err := user.appLogin("ios", newClient); err != nil {
		t.Fatal(err)
	}
	if err := user.appLogout("ios", oldClient); err != nil {
		t.Fatal(err)
	}

	subs, unsubs, _ := transport.counts("$user:B")
	if subs != 1 || unsubs != 0 {
		t.Fatalf("replacement subscribe/unsubscribe = %d/%d, want 1/0", subs, unsubs)
	}
	if user.ClientCount() != 1 {
		t.Fatalf("client count after replacement = %d, want 1", user.ClientCount())
	}
}

func TestClusterUserReleasesAndReacquiresRetainedTopics(t *testing.T) {
	clearClusterTransport()
	t.Cleanup(clearClusterTransport)
	transport := newFakeClusterTransport()
	setClusterTransport(transport)

	user := newClusterLifecycleUser("B")
	user.SubTopics["room:1"] = &Topic{Id: "room:1"}
	first := &Client{}

	if err := user.appLogin("ios", first); err != nil {
		t.Fatal(err)
	}
	roomSubs, roomUnsubs, _ := transport.counts("room:1")
	if roomSubs != 1 || roomUnsubs != 0 {
		t.Fatalf("retained topic after login = %d/%d, want 1/0", roomSubs, roomUnsubs)
	}

	if err := user.appLogout("ios", first); err != nil {
		t.Fatal(err)
	}
	roomSubs, roomUnsubs, _ = transport.counts("room:1")
	if roomSubs != 1 || roomUnsubs != 1 {
		t.Fatalf("retained topic after logout = %d/%d, want 1/1", roomSubs, roomUnsubs)
	}
	if _, ok := user.SubTopics["room:1"]; !ok {
		t.Fatal("logout removed retained local topic")
	}

	second := &Client{}
	if err := user.appLogin("ios", second); err != nil {
		t.Fatal(err)
	}
	roomSubs, roomUnsubs, _ = transport.counts("room:1")
	if roomSubs != 2 || roomUnsubs != 1 {
		t.Fatalf("retained topic after reconnect = %d/%d, want 2/1", roomSubs, roomUnsubs)
	}
}
