package ws

type TopicMsg struct {
	Ori     any    //原始数据方便订阅主题的函数处理
	TopicId string //话题ID
	Msg     []byte //消息内容，方便客户端处理
}
