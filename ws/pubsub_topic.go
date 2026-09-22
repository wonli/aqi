package ws

import (
	"sync"
	"time"
)

type Topic struct {
	Id          string   //订阅主题ID
	PubSub      *PubSub  //关联PubSub
	SubUsers    sync.Map //SubUsers map[string]*time.Time //订阅用户uniqueId和订阅时间
	SubHandlers sync.Map //SubHandlers map[string]func(msg *TopicMsg) //内部组件间通知
}

func (a *Topic) addSubUser(user *User) bool {
	if user == nil {
		return false
	}

	user.Lock()
	_, loaded := user.SubTopics[a.Id]
	a.SubUsers.LoadOrStore(user.Suid, time.Now())
	user.SubTopics[a.Id] = a
	user.Unlock()
	clusterSyncUser(user, clusterTopicChannel(a.Id))
	return !loaded
}

// AddSubUser preserves the existing public API; cluster transition details stay internal.
func (a *Topic) AddSubUser(user *User) {
	a.addSubUser(user)
}

func (a *Topic) AddSubHandle(f func(msg *TopicMsg)) {
	a.SubHandlers.LoadOrStore(a.Id, f)
}

// RemoveSubUser 从主题订阅集合中移除指定用户
func (a *Topic) RemoveSubUser(suid string) {
	a.SubUsers.Delete(suid)
}

func (a *Topic) SendToSubUser(msg *TopicMsg) {
	var data []byte
	a.SubUsers.Range(func(key, value any) bool {
		uniqueId := key.(string)
		user := Hub.User(uniqueId)
		if user != nil {
			if data == nil {
				data = msg.encode()
				if data == nil {
					return false
				}
			}
			user.SendMsg(data)
		}

		return true
	})
}

func (a *Topic) ApplyFunc(msg *TopicMsg) {
	a.SubHandlers.Range(func(key, value any) bool {
		f := value.(func(msg *TopicMsg))
		f(msg)
		return true
	})
}
