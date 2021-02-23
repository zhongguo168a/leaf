package gate

import (
	"bytes"
	"encoding/binary"
	"github.com/zhongguo168a/leaf/chanrpc"
	"github.com/zhongguo168a/leaf/network"
	"net"
	"reflect"
	"time"
	"zhongguo168a.top/mycodes/gocodes/ezcache"
	"zhongguo168a.top/mycodes/gocodes/mylog"
	"zhongguo168a.top/mycodes/gocodes/utils/binaryutil"
)

type Gate struct {
	MaxConnNum      int
	PendingWriteNum int
	MaxMsgLen       uint32
	Processor       network.Processor
	AgentChanRPC    *chanrpc.Server

	// websocket
	WSAddr      string
	HTTPTimeout time.Duration
	CertFile    string
	KeyFile     string

	// tcp
	TCPAddr      string
	LenMsgLen    int
	LittleEndian bool
}

func (gate *Gate) Run(closeSig chan bool) {
	var wsServer *network.WSServer
	if gate.WSAddr != "" {
		wsServer = new(network.WSServer)
		wsServer.Addr = gate.WSAddr
		wsServer.MaxConnNum = gate.MaxConnNum
		wsServer.PendingWriteNum = gate.PendingWriteNum
		wsServer.MaxMsgLen = gate.MaxMsgLen
		wsServer.HTTPTimeout = gate.HTTPTimeout
		wsServer.CertFile = gate.CertFile
		wsServer.KeyFile = gate.KeyFile
		wsServer.NewAgent = func(conn *network.WSConn) network.Agent {
			a := &agent{conn: conn, gate: gate}
			if gate.AgentChanRPC != nil {
				gate.AgentChanRPC.Go("NewAgent", a)
			}
			return a
		}
	}

	var tcpServer *network.TCPServer
	if gate.TCPAddr != "" {
		tcpServer = new(network.TCPServer)
		tcpServer.Addr = gate.TCPAddr
		tcpServer.MaxConnNum = gate.MaxConnNum
		tcpServer.PendingWriteNum = gate.PendingWriteNum
		tcpServer.LenMsgLen = gate.LenMsgLen
		tcpServer.MaxMsgLen = gate.MaxMsgLen
		tcpServer.LittleEndian = gate.LittleEndian
		tcpServer.NewAgent = func(conn *network.TCPConn) network.Agent {
			a := &agent{conn: conn, gate: gate}
			if gate.AgentChanRPC != nil {
				gate.AgentChanRPC.Go("NewAgent", a)
			}
			return a
		}
	}

	if wsServer != nil {
		wsServer.Start()
	}
	if tcpServer != nil {
		tcpServer.Start()
	}
	<-closeSig
	if wsServer != nil {
		wsServer.Close()
	}
	if tcpServer != nil {
		tcpServer.Close()
	}
}

func (gate *Gate) OnDestroy() {}

type agent struct {
	conn     network.Conn
	gate     *Gate
	userData interface{}
	// 来自客户端
	lastSeq int
	// agent自身, 协议派发到客户端的序列号, 用于调试
	seqSend int16
	//
	cache *ezcache.Cache
}

func (a *agent) Run() {
	a.cache = &ezcache.Cache{}
	for {
		data, err := a.conn.ReadMsg()
		if err != nil {
			mylog.Debug("read message: %v", err)
			break
		}

		if len(data) < 4 {
			mylog.Debug("read message: invalid bytes")
			break
		}

		reader := bytes.NewReader(data)
		seq := int(binaryutil.ReadInt16(reader, binary.BigEndian))
		msgId := binaryutil.ReadUTF(reader, binary.BigEndian)
		msgString := binaryutil.ReadUTF(reader, binary.BigEndian)
		mylog.Debug("<<< seq=%v, msg=%v, data=%v\n", seq, msgId, msgString)

		if a.gate.Processor != nil {
			msg, perr := a.gate.Processor.Unmarshal(msgId, []byte(msgString))
			if perr != nil {
				mylog.Debug("unmarshal message error: %v", perr)
				break
			}
			rerr := a.gate.Processor.Route(map[string]interface{}{
				"msgId": msgId,
				"data":  msg,
				"seq":   seq,
			}, a)
			if rerr != nil {
				mylog.Debug("route message error: %v", rerr)
				break
			}
		}
	}
}

func (a *agent) OnClose() {
	if a.gate.AgentChanRPC != nil {
		err := a.gate.AgentChanRPC.Call0("CloseAgent", a)
		if err != nil {
			mylog.Error("chanrpc error: %v", err)
		}
	}
}

func (a *agent) SetLastSeq(val int) {
	a.lastSeq = val
}

func (a *agent) WriteMsg(msg interface{}) {
	a.seqSend++

	buff := bytes.NewBuffer([]byte{})
	binary.Write(buff, binary.BigEndian, int16(a.lastSeq))
	binary.Write(buff, binary.BigEndian, int16(a.seqSend))
	binary.Write(buff, binary.BigEndian, msg.([]byte))
	err := a.conn.WriteMsg(buff.Bytes())
	if err != nil {
		mylog.Error("write message %v error: %v", reflect.TypeOf(msg), err)
	}
}

func (a *agent) LocalAddr() net.Addr {
	return a.conn.LocalAddr()
}

func (a *agent) RemoteAddr() net.Addr {
	return a.conn.RemoteAddr()
}

func (a *agent) Close() {
	a.conn.Close()
}

func (a *agent) Destroy() {
	a.conn.Destroy()
}

func (a *agent) Cache() *ezcache.Cache {
	return a.cache
}

func (a *agent) UserData() interface{} {
	return a.userData
}

func (a *agent) SetUserData(data interface{}) {
	a.userData = data
}
