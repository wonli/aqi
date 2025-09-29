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

func (a *Topic) AddSubUser(user *User) {
	user.AddSubTopic(a)
	a.SubUsers.LoadOrStore(user.Suid, time.Now())
}

func (a *Topic) AddSubHandle(f func(msg *TopicMsg)) {
    a.SubHandlers.LoadOrStore(a.Id, f)
}

// RemoveSubUser 从主题订阅集合中移除指定用户
func (a *Topic) RemoveSubUser(suid string) {
    a.SubUsers.Delete(suid)
}

func (a *Topic) SendToSubUser(msg []byte) {
	a.SubUsers.Range(func(key, value any) bool {
		uniqueId := key.(string)
		user := Hub.User(uniqueId)
		if user != nil {
			user.SendMsg(msg)
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
