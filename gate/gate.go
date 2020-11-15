package gate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/zhongguo168a/leaf/chanrpc"
	"github.com/zhongguo168a/leaf/log"
	"github.com/zhongguo168a/leaf/network"
	"net"
	"reflect"
	"time"
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
}

func (a *agent) Run() {
	for {
		data, err := a.conn.ReadMsg()
		if err != nil {
			log.Debug("read message: %v", err)
			break
		}

		if len(data) < 4 {
			log.Debug("read message: invalid bytes")
			break
		}

		reader := bytes.NewReader(data)
		seq := int(binaryutil.ReadInt16(reader, binary.BigEndian))
		msgId := binaryutil.ReadUTF(reader, binary.BigEndian)
		msgString := binaryutil.ReadUTF(reader, binary.BigEndian)
		fmt.Printf("<<< seq=%v, msg=%v, data=%v\n", seq, msgId, msgString)

		if a.gate.Processor != nil {
			msg, err := a.gate.Processor.Unmarshal(msgId, []byte(msgString))
			if err != nil {
				log.Debug("unmarshal message error: %v", err)
				break
			}
			err = a.gate.Processor.Route(map[string]interface{}{
				"msgId": msgId,
				"data":  msg,
				"seq":   seq,
			}, a)
			if err != nil {
				log.Debug("route message error: %v", err)
				wb := &bytes.Buffer{}
				binary.Write(wb, binary.BigEndian, int16(0))
				binary.Write(wb, binary.BigEndian, int16(1))
				binary.Write(wb, binary.BigEndian, int16(0))
				binary.Write(wb, binary.BigEndian, int16(-1))
				a.WriteMsg(wb.Bytes())
				break
			}
		}
	}
}

func (a *agent) OnClose() {
	if a.gate.AgentChanRPC != nil {
		err := a.gate.AgentChanRPC.Call0("CloseAgent", a)
		if err != nil {
			log.Error("chanrpc error: %v", err)
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
	binary.Write(buff, binary.BigEndian,  msg.([]byte))
	err := a.conn.WriteMsg(buff.Bytes())
	if err != nil {
		log.Error("write message %v error: %v", reflect.TypeOf(msg), err)
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

func (a *agent) UserData() interface{} {
	return a.userData
}

func (a *agent) SetUserData(data interface{}) {
	a.userData = data
}
