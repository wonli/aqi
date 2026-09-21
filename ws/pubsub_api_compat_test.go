package ws

// These assignments intentionally pin the public method signatures from main.
// Cluster support may add internal transition helpers, but must not change
// existing framework API types as a side effect.
var (
	_ func(*PubSub, string, *User) = (*PubSub).Sub
	_ func(*PubSub, string, *User) = (*PubSub).Unsub
	_ func(*Topic, *User)          = (*Topic).AddSubUser
	_ func(*Topic, string)         = (*Topic).RemoveSubUser
)
