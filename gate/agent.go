package gate

import (
	"net"
	"zhongguo168a.top/mycodes/gocodes/ezcache"
)

type Agent interface {
	// 设置序号
	SetLastSeq(val int)
	WriteMsg(msg interface{})
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close()
	Destroy()
	Cache() *ezcache.Cache
	UserData() interface{}
	SetUserData(data interface{})
}
