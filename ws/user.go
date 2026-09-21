package ws

import (
	"sync"
	"time"

	"golang.org/x/exp/slices"
)

type User struct {
	//公共基础信息
	Uid          uint             `json:"uid"`                //整型唯一ID
	Suid         string           `json:"suid"`               //字符唯一ID
	GroupId      string           `json:"groupId"`            //分组ID
	SuperAdmin   bool             `json:"superAdmin"`         //是否超管
	RoleId       []uint           `json:"roleId,omitempty"`   //用户角色
	Nickname     string           `json:"nickname"`           //昵称
	Avatar       *Resource        `json:"avatar"`             //用户头像
	OnlineStatus UserOnlineStatus `json:"onlineStatus"`       //在线状态
	Location     *Location        `json:"location,omitempty"` //地理位置

	CurrentWindowId string //当前的窗口ID

	//禁言时间
	Ban *time.Time `json:"ban,omitempty"`

	//最后心跳时间
	LastHeartbeatTime time.Time

	//用户相关数据
	Hub        *Hubc     `json:"-"`
	AppClients []*Client `json:"-"` //appId对应客户端

	SubTopics    map[string]*Topic `json:"-"` //topicId订阅的主题名称及信息
	sync.RWMutex `json:"-"`
}

func NewUser(uid string) *User {
	user := &User{
		Suid:       uid,
		Hub:        Hub,
		AppClients: []*Client{},

		SubTopics: make(map[string]*Topic),
	}

	return user
}

func (u *User) AddSubTopic(topic *Topic) int {
	u.Lock()
	defer u.Unlock()

	u.SubTopics[topic.Id] = topic
	return len(u.SubTopics)
}

func (u *User) unsubscribeTopic(topicId string) (int, bool) {
	u.Lock()
	topic, ok := u.SubTopics[topicId]
	online := len(u.AppClients) > 0
	if ok {
		delete(u.SubTopics, topicId)
	}
	remaining := len(u.SubTopics)
	u.Unlock()

	if !ok {
		return remaining, false
	}
	if topic != nil {
		topic.RemoveSubUser(u.Suid)
	}
	if online && clusterEnabled() {
		clusterRelease(clusterTopicChannel(topicId))
	}
	return remaining, true
}

func (u *User) UnsubTopic(topicId string) int {
	remaining, _ := u.unsubscribeTopic(topicId)
	return remaining
}

func (u *User) UnsubAllTopics() int {
	u.Lock()
	online := len(u.AppClients) > 0
	topics := make([]*Topic, 0, len(u.SubTopics))
	for topicId, topic := range u.SubTopics {
		if topic != nil {
			topics = append(topics, topic)
		}
		delete(u.SubTopics, topicId)
	}
	remaining := len(u.SubTopics)
	u.Unlock()

	distributed := online && clusterEnabled()
	for _, topic := range topics {
		topic.RemoveSubUser(u.Suid)
		if distributed {
			clusterRelease(clusterTopicChannel(topic.Id))
		}
	}

	return remaining
}

func (u *User) subTopicIDsLocked() []string {
	topics := make([]string, 0, len(u.SubTopics))
	for topicID := range u.SubTopics {
		topics = append(topics, topicID)
	}
	return topics
}

// SetLastHeartbeat updates the user's latest client heartbeat time.
func (u *User) SetLastHeartbeat(t time.Time) {
	if u == nil {
		return
	}
	u.Lock()
	u.LastHeartbeatTime = t
	u.Unlock()
}

// LastHeartbeat returns the user's latest client heartbeat time.
func (u *User) LastHeartbeat() time.Time {
	if u == nil {
		return time.Time{}
	}
	u.RLock()
	defer u.RUnlock()
	return u.LastHeartbeatTime
}

// AppLogin 用户APP客户端登录
func (u *User) appLogin(appId string, client *Client) error {
	var replacedClient *Client
	distributed := clusterEnabled()

	u.Lock()
	wasOffline := len(u.AppClients) == 0
	matchedApp := false
	for i, app := range u.AppClients {
		_, existingAppId, _ := app.LoginState()
		if existingAppId != appId {
			continue
		}

		matchedApp = true
		if app.Conn != client.Conn {
			replacedClient = app
			u.AppClients = slices.Delete(u.AppClients, i, i+1)
			u.AppClients = append(u.AppClients, client)
		}
		break
	}
	if !matchedApp {
		u.AppClients = append(u.AppClients, client)
	}
	client.setLoginState(u, appId)
	becameOnline := wasOffline && len(u.AppClients) > 0
	var retainedTopics []string
	if distributed && becameOnline {
		retainedTopics = u.subTopicIDsLocked()
	}
	u.Unlock()

	if distributed && becameOnline {
		userChannel := clusterUserChannel(u.Suid)
		clusterAcquire(userChannel)
		if !u.IsOnline() {
			clusterRelease(userChannel)
		}

		for _, topicID := range retainedTopics {
			topicChannel := clusterTopicChannel(topicID)
			clusterAcquire(topicChannel)

			u.RLock()
			_, stillSubscribed := u.SubTopics[topicID]
			stillOnline := len(u.AppClients) > 0
			u.RUnlock()
			if !stillSubscribed || !stillOnline {
				clusterRelease(topicChannel)
			}
		}
	}

	if replacedClient != nil {
		replacedClient.Disconnect()
	}
	u.Hub.PubSub.Pub("login", u)
	return nil
}

// app退出
func (u *User) appLogout(appId string, logoutClient *Client) error {
	distributed := clusterEnabled()

	u.Lock()
	removed := false
	for appIndex, appClient := range u.AppClients {
		_, existingAppId, _ := appClient.LoginState()
		if existingAppId == appId && logoutClient.Conn == appClient.Conn {
			u.AppClients = slices.Delete(u.AppClients, appIndex, appIndex+1)
			removed = true
			break
		}
	}
	becameOffline := removed && len(u.AppClients) == 0
	var retainedTopics []string
	if distributed && becameOffline {
		retainedTopics = u.subTopicIDsLocked()
	}
	u.Unlock()

	if distributed && becameOffline {
		clusterRelease(clusterUserChannel(u.Suid))
		for _, topicID := range retainedTopics {
			clusterRelease(clusterTopicChannel(topicID))
		}
	}

	u.Hub.PubSub.Pub("logout", u)
	return nil
}

// AppClient 获取APP客户端
func (u *User) AppClient(appId string) *Client {
	if u == nil {
		return nil
	}

	u.RLock()
	defer u.RUnlock()

	for _, app := range u.AppClients {
		_, existingAppId, _ := app.LoginState()
		if existingAppId == appId {
			return app
		}
	}

	return nil
}

func (u *User) ClientCount() int {
	if u == nil {
		return 0
	}

	u.RLock()
	defer u.RUnlock()
	return len(u.AppClients)
}

func (u *User) IsBanned() (bool, *time.Time) {
	u.RLock()
	defer u.RUnlock()
	if u.Ban == nil || u.Ban.IsZero() {
		return false, nil
	}
	ban := *u.Ban
	return true, &ban
}

func (u *User) Banned(t time.Duration) *time.Time {
	banTime := time.Now().Add(t)
	u.Lock()
	u.Ban = &banTime
	u.Unlock()
	return &banTime
}

func (u *User) Unban() *time.Time {
	u.Lock()
	u.Ban = nil
	u.Unlock()
	return nil
}

func (u *User) IsOnline() bool {
	return u.ClientCount() > 0
}

func (u *User) SendMsg(msg []byte) {
	if u == nil {
		return
	}

	u.RLock()
	clients := append([]*Client(nil), u.AppClients...)
	u.RUnlock()

	for _, client := range clients {
		client.SendMsg(msg)
	}
}

func (u *User) SendMsgToApp(appId string, msg []byte) {
	client := u.AppClient(appId)
	if client != nil {
		client.SendMsg(msg)
	}
}

func (u *User) sendAction(action *Action) {
	if u == nil {
		return
	}

	f, err := actionFrame(action)
	if err != nil {
		return
	}

	u.RLock()
	clients := append([]*Client(nil), u.AppClients...)
	u.RUnlock()

	for _, client := range clients {
		client.sendFrame(f)
	}
}

func (u *User) sendActionToApp(appId string, action *Action) {
	client := u.AppClient(appId)
	if client != nil {
		client.SendActionMsg(action)
	}
}
