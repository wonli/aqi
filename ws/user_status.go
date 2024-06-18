package ws

type UserOnlineStatus byte

const (
	UserStatusOnline UserOnlineStatus = iota
	UserStatusBusy
	UserStatusLeaving
)

func IsValidStatus(s UserOnlineStatus) bool {
	return s <= UserStatusLeaving
}
