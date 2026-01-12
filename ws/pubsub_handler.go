package ws

func Pub(topic string, data any) {
	Hub.PubSub.Pub(topic, data)
}

func Sub(topicId string, user *User) {
	Hub.PubSub.Sub(topicId, user)
}

func SubFunc(topicId string, f func(msg *TopicMsg)) {
	Hub.PubSub.SubFunc(topicId, f)
}

func Unsub(topicId string, user *User) {
	Hub.PubSub.Unsub(topicId, user)
}
