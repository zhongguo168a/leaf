package gate

import (
	"net"
)

type Agent interface {
	// 设置序号
	SetLastSeq(val int)
	WriteMsg(msg interface{})
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close()
	Destroy()
	UserData() interface{}
	SetUserData(data interface{})
}
