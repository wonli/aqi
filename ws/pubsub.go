package ws

import (
	"sync"

	"github.com/wonli/aqi/logger"
)

type PubSub struct {
	Topics        *sync.Map      //Topics map[string]*Topic //主题名称和Top对应map
	TopicMsgQueue chan *TopicMsg //主题消息队列
}

func NewPubSub() *PubSub {
	return &PubSub{
		Topics:        new(sync.Map),
		TopicMsgQueue: make(chan *TopicMsg, 128),
	}
}

func (a *PubSub) initTopic(topicId string) *Topic {
	candidate := &Topic{
		Id:          topicId,
		PubSub:      a,
		SubUsers:    sync.Map{},
		SubHandlers: sync.Map{},
	}

	topic, _ := a.Topics.LoadOrStore(topicId, candidate)
	return topic.(*Topic)
}

func (a *PubSub) topicMsg(topicId string, data any) *TopicMsg {
	a.initTopic(topicId)
	return &TopicMsg{
		Ori:     data,
		TopicId: topicId,
	}
}

// Pub 发布进程内通知。队列已满时丢弃当前通知，不阻塞调用方。
// PubSub 是 best-effort 通知机制，不应用于需要可靠执行的关键业务任务。
func (a *PubSub) Pub(topicId string, data any) bool {
	msg := a.topicMsg(topicId, data)
	select {
	case a.TopicMsgQueue <- msg:
		return true
	default:
		return false
	}
}

// Sub 订阅主题。返回值表示是否新增了用户/主题订阅关系。
func (a *PubSub) Sub(topicId string, user *User) bool {
	if user == nil {
		return false
	}

	added := a.initTopic(topicId).AddSubUser(user)
	if added && user.IsOnline() {
		clusterAcquire(topicId)
	}
	return added
}

// SubFunc 以函数方式订阅
func (a *PubSub) SubFunc(topicId string, f func(msg *TopicMsg)) {
	a.initTopic(topicId).AddSubHandle(f)
}

// Unsub 取消订阅主题。返回值表示订阅关系是否实际发生了移除。
func (a *PubSub) Unsub(topicId string, user *User) bool {
	if user == nil {
		return false
	}

	topic, removed := user.removeSubTopic(topicId)
	if !removed {
		return false
	}
	if topic != nil {
		topic.RemoveSubUser(user.Suid)
	}
	if user.IsOnline() {
		clusterRelease(topicId)
	}
	return true
}

func (a *PubSub) Start() {
	for msg := range a.TopicMsgQueue {
		t, hasTopic := a.Topics.Load(msg.TopicId)
		if !hasTopic {
			logger.SugarLog.Info("未发布订阅主题收到消息")
			continue
		}

		//订阅消息的函数处理
		t.(*Topic).ApplyFunc(msg)

		//订阅消息的用户处理
		t.(*Topic).SendToSubUser(msg)
	}
}
