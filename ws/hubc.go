package ws

import (
	"sync"
	"time"
)

var Hub *Hubc

type Hubc struct {
	//所有物理 WebSocket 连接
	Clients map[*Client]struct{}
	clientsMu sync.RWMutex

	//已登录用户 map[string]*User
	Users *sync.Map

	//用户数统计
	LoginCount int
	GuestCount int

	//发布订阅
	PubSub *PubSub

	//登录和断开通道
	Connection chan *Client
	Disconnect chan *Client
}

type GuardFunc func(h *Hubc)

// GuardFunc 守护回调
var guardFn GuardFunc

// SetGuardFunc 设置全局的 Hubc 守护回调
func SetGuardFunc(fn GuardFunc) {
	guardFn = fn
}

func NewHubc() *Hubc {
	Hub = &Hubc{
		PubSub:     NewPubSub(),
		Clients:    make(map[*Client]struct{}),
		Users:      new(sync.Map),
		Connection: make(chan *Client, 256),
		Disconnect: make(chan *Client, 256),
	}

	return Hub
}

func (h *Hubc) Run() {
	go h.PubSub.Start()
	go h.guard()

	for {
		select {
		case c := <-h.Connection:
			h.clientsMu.Lock()
			h.Clients[c] = struct{}{}
			h.clientsMu.Unlock()

			h.PubSub.Pub("connect", c)
			c.Log("--", "connection")

		case c := <-h.Disconnect:
			h.clientsMu.Lock()
			delete(h.Clients, c)
			h.clientsMu.Unlock()

			h.PubSub.Pub("disconnect", c)
			if c.User != nil {
				err := c.User.appLogout(c.AppId, c)
				if err != nil {
					c.Log("--", "user disconnect err:"+err.Error())
				}
			}
		}
	}
}

func (h *Hubc) guard() {
	cleanupTTL := 5 * time.Minute
	timer := time.NewTicker(30 * time.Second)
	defer timer.Stop()

	for range timer.C {
		if guardFn != nil {
			guardFn(h)
		}

		userCount := 0
		h.Users.Range(func(key, value any) bool {
			user, ok := value.(*User)
			if !ok || user == nil {
				return true
			}

			if len(user.AppClients) == 0 {
				if time.Since(user.LastHeartbeatTime) >= cleanupTTL {
					user.UnsubAllTopics()
					h.Users.Delete(key)
					h.PubSub.Pub("cleanupUser", H{"suid": user.Suid})
				}
			} else {
				userCount++
			}

			return true
		})

		guestCount := 0
		h.clientsMu.RLock()
		for client := range h.Clients {
			if !client.IsLogin {
				guestCount++
			}
		}
		h.clientsMu.RUnlock()

		//登录用户数
		h.LoginCount = userCount
		h.GuestCount = guestCount

		//发布订阅消息
		h.PubSub.Pub("userCount", userCount)
		h.PubSub.Pub("guestsCount", guestCount)
	}
}

// Broadcast 发送广播消息
func (h *Hubc) Broadcast(msg []byte) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for client := range h.Clients {
		client.SendMsg(msg)
	}
}

// User 获取用户信息
func (h *Hubc) User(uid string) *User {
	user, ok := h.Users.Load(uid)
	if ok {
		return user.(*User)
	}

	return nil
}

// UserClient 获取用户客户端信息
func (h *Hubc) UserClient(uid, appId string) *Client {
	user := h.User(uid)
	if user != nil {
		return user.AppClient(appId)
	}

	return nil
}

// UserLogin 用户登录
func (h *Hubc) UserLogin(uid, appId string, client *Client) error {
	user := h.User(uid)
	if user == nil {
		user = NewUser(uid)
	}

	//app登录
	err := user.appLogin(appId, client)
	if err != nil {
		return err
	}

	//保存用户
	h.Users.Store(uid, user)
	return nil
}
