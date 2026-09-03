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
	msg := Action{
		Action: topicId,
		Data: H{
			"topicId": topicId,
			"message": data,
		},
	}

	a.initTopic(topicId)
	return &TopicMsg{
		Ori:     data,
		TopicId: topicId,
		Msg:     msg.Encode(),
	}
}

// Pub 发布主题。业务主动发布保留阻塞语义，以提供队列背压。
func (a *PubSub) Pub(topicId string, data any) {
	a.TopicMsgQueue <- a.topicMsg(topicId, data)
}

// TryPub 非阻塞发布主题。用于框架生命周期/状态通知，避免 PubSub 背压阻塞核心流程。
// 返回 false 表示队列已满，当前通知未入队。
func (a *PubSub) TryPub(topicId string, data any) bool {
	msg := a.topicMsg(topicId, data)
	select {
	case a.TopicMsgQueue <- msg:
		return true
	default:
		return false
	}
}

// Sub 订阅主题
func (a *PubSub) Sub(topicId string, user *User) {
	a.initTopic(topicId).AddSubUser(user)
}

// SubFunc 以函数方式订阅
func (a *PubSub) SubFunc(topicId string, f func(msg *TopicMsg)) {
	a.initTopic(topicId).AddSubHandle(f)
}

// Unsub 取消订阅主题
func (a *PubSub) Unsub(topicId string, user *User) {
	topic, ok := a.Topics.Load(topicId)
	if ok {
		topic.(*Topic).RemoveSubUser(user.Suid)
		user.UnsubTopic(topicId)
	}
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
		t.(*Topic).SendToSubUser(msg.Msg)
	}
}
