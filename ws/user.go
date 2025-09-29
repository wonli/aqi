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

	SubTopics map[string]*Topic `json:"-"` //topicId订阅的主题名称及信息
	sync.RWMutex
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

func (u *User) UnsubTopic(topicId string) int {
	u.Lock()
	defer u.Unlock()

	_, ok := u.SubTopics[topicId]
	if ok {
		delete(u.SubTopics, topicId)
	}

	return len(u.SubTopics)
}

// UnsubAllTopics 取消用户的所有主题订阅（用户侧与主题侧同时清理）
func (u *User) UnsubAllTopics() int {
	u.Lock()
	defer u.Unlock()

	for topicId, topic := range u.SubTopics {
		if topic != nil {
			// 从主题订阅集合中移除该用户
			topic.RemoveSubUser(u.Suid)
		}

		// 从用户侧映射移除该主题
		delete(u.SubTopics, topicId)
	}

	return len(u.SubTopics)
}

// AppLogin 用户APP客户端登录
func (u *User) appLogin(appId string, client *Client) error {
	var index int
	var appClient *Client
	for i, app := range u.AppClients {
		if app.AppId == appId {
			index = i
			appClient = app
			break
		}
	}

	client.User = u
	client.AppId = appId
	client.IsLogin = true
	if appClient != nil {
		if appClient.Conn != client.Conn {
			u.AppClients = slices.Delete(u.AppClients, index, index+1)
			u.AppClients = append(u.AppClients, client)

			//已登录连接下线
			u.Hub.Disconnect <- appClient
		}
	} else {
		u.AppClients = append(u.AppClients, client)
	}

	u.Hub.PubSub.Pub("login", u)
	return nil
}

// app退出
func (u *User) appLogout(appId string, logoutClient *Client) error {
	removeIndex := -1
	for appIndex, appClient := range u.AppClients {
		if appClient.AppId == appId && logoutClient.Conn == appClient.Conn {
			removeIndex = appIndex
			break
		}
	}

	if removeIndex > -1 {
		//从客户端中移除
		u.AppClients = slices.Delete(u.AppClients, removeIndex, removeIndex+1)

		//关闭客户端
		logoutClient.Close()
	}

	u.Hub.PubSub.Pub("logout", u)
	return nil
}

// AppClient 获取APP客户端
func (u *User) AppClient(appId string) *Client {
	for _, app := range u.AppClients {
		cc := app
		if cc.AppId == appId {
			return cc
		}
	}

	return nil
}

// IsBanned 是否被封禁
func (u *User) IsBanned() (bool, *time.Time) {
	if u.Ban == nil || u.Ban.IsZero() {
		return false, nil
	}

	return true, u.Ban
}

// Banned 禁言用户
func (u *User) Banned(t time.Duration) *time.Time {
	banTime := time.Now().Add(t)
	u.Ban = &banTime
	return u.Ban
}

// Unban 禁言解除
func (u *User) Unban() *time.Time {
	u.Ban = nil
	return u.Ban
}

// IsOnline 用户是否在线
func (u *User) IsOnline() bool {
	if u == nil || u.AppClients == nil {
		return false
	}

	return len(u.AppClients) > 0
}

// SendMsg 发送消息
func (u *User) SendMsg(msg []byte) {
	if u == nil {
		return
	}

	for _, client := range u.AppClients {
		client.SendMsg(msg)
	}
}

// SendMsgToApp 发送消息到指定客户端
func (u *User) SendMsgToApp(appId string, msg []byte) {
	client := u.AppClient(appId)
	if client != nil {
		client.SendMsg(msg)
	}
}
