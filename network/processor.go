package network

type Processor interface {
	// must goroutine safe
	Route(msg interface{}, userData interface{}) error
	// must goroutine safe
	Unmarshal(msgId string, data []byte) (interface{}, error)
	// must goroutine safe
	Marshal(msg interface{}) (msgId string, data []byte, err error)
	// 处理错误
	HandleError(err error, msg interface{}, userData interface{})
}
