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

func (a *PubSub) enqueue(msg *TopicMsg) bool {
	select {
	case a.TopicMsgQueue <- msg:
		return true
	default:
		return false
	}
}

// Pub 发布进程内通知。队列已满时丢弃当前通知，不阻塞调用方。
// PubSub 是 best-effort 通知机制，不应用于需要可靠执行的关键业务任务。
func (a *PubSub) Pub(topicId string, data any) bool {
	return a.enqueue(a.topicMsg(topicId, data))
}

// Publish 发布到本机并将同一编码后的消息发送到其他 AQI 节点。
// 可靠存储仍由业务层负责。
func (a *PubSub) Publish(topicId string, data any) bool {
	if !clusterEnabled() {
		return a.Pub(topicId, data)
	}

	msg := a.topicMsg(topicId, data)
	encoded := msg.encode()
	local := a.enqueue(msg)
	remote := encoded != nil && clusterPublish(clusterTopicChannel(topicId), encoded)
	return local || remote
}

// deliverCluster 只投递 Redis 入站消息到本机订阅用户，不触发 SubFunc，也不再次发布到 cluster。
func (a *PubSub) deliverCluster(topicId string, data []byte) bool {
	if a == nil || a.Topics == nil {
		return false
	}
	topicValue, ok := a.Topics.Load(topicId)
	if !ok {
		return false
	}
	topicValue.(*Topic).SendToSubUser(&TopicMsg{
		TopicId: topicId,
		Msg:     append([]byte(nil), data...),
	})
	return true
}

func (a *PubSub) subscribe(topicId string, user *User) bool {
	if user == nil {
		return false
	}

	added, online := a.initTopic(topicId).addSubUser(user)
	if added && online && clusterEnabled() {
		clusterAcquire(clusterTopicChannel(topicId))
	}
	return added
}

// Sub 订阅主题
func (a *PubSub) Sub(topicId string, user *User) {
	a.subscribe(topicId, user)
}

// SubFunc 以函数方式订阅
func (a *PubSub) SubFunc(topicId string, f func(msg *TopicMsg)) {
	a.initTopic(topicId).AddSubHandle(f)
}

func (a *PubSub) unsubscribe(topicId string, user *User) bool {
	if user == nil {
		return false
	}

	_, removed := user.unsubscribeTopic(topicId)
	return removed
}

// Unsub 取消订阅主题
func (a *PubSub) Unsub(topicId string, user *User) {
	a.unsubscribe(topicId, user)
}

func (a *PubSub) Start() {
	for msg := range a.TopicMsgQueue {
		t, hasTopic := a.Topics.Load(msg.TopicId)
		if !hasTopic {
			if logger.SugarLog != nil {
				logger.SugarLog.Info("未发布订阅主题收到消息")
			}
			continue
		}

		//订阅消息的函数处理
		t.(*Topic).ApplyFunc(msg)

		//订阅消息的用户处理
		t.(*Topic).SendToSubUser(msg)
	}
}
