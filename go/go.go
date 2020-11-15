package g

import (
	"container/list"
	"github.com/zhongguo168a/leaf/conf"
	"github.com/zhongguo168a/leaf/log"
	"runtime"
	"sync"
)

// one Go per goroutine (goroutine not safe)
type Go struct {
	ChanCb    chan CbContext
	pendingGo int
}

type LinearGo struct {
	f  func()
	cb CbContext
}

type CbContext struct {
	f    func(ret interface{})
	args interface{}
}

type LinearContext struct {
	g              *Go
	linearGo       *list.List
	mutexLinearGo  sync.Mutex
	mutexExecution sync.Mutex
}

func New(l int) *Go {
	g := new(Go)
	g.ChanCb = make(chan CbContext, l)
	return g
}

func (g *Go) Go(f func() interface{}, cb func(ret interface{})) {
	g.pendingGo++
	
	go func() {
		var ret CbContext
		ret.f = cb
		defer func() {
			g.ChanCb <- ret
			if r := recover(); r != nil {
				if conf.LenStackBuf > 0 {
					buf := make([]byte, conf.LenStackBuf)
					l := runtime.Stack(buf, false)
					log.Error("%v: %s", r, buf[:l])
				} else {
					log.Error("%v", r)
				}
			}
		}()
		
		ret.args = f()
	}()
}

func (g *Go) Cb(ret CbContext) {
	defer func() {
		g.pendingGo--
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Error("%v: %s", r, buf[:l])
			} else {
				log.Error("%v", r)
			}
		}
	}()
	
	ret.f(ret.args)
}

func (g *Go) Close() {
	for g.pendingGo > 0 {
		g.Cb(<-g.ChanCb)
	}
}

func (g *Go) Idle() bool {
	return g.pendingGo == 0
}

func (g *Go) NewLinearContext() *LinearContext {
	c := new(LinearContext)
	c.g = g
	c.linearGo = list.New()
	return c
}

func (c *LinearContext) Go(f func(), cb CbContext) {
	c.g.pendingGo++
	
	c.mutexLinearGo.Lock()
	c.linearGo.PushBack(&LinearGo{f: f, cb: cb})
	c.mutexLinearGo.Unlock()
	
	go func() {
		c.mutexExecution.Lock()
		defer c.mutexExecution.Unlock()
		
		c.mutexLinearGo.Lock()
		e := c.linearGo.Remove(c.linearGo.Front()).(*LinearGo)
		c.mutexLinearGo.Unlock()
		
		defer func() {
			c.g.ChanCb <- e.cb
			if r := recover(); r != nil {
				if conf.LenStackBuf > 0 {
					buf := make([]byte, conf.LenStackBuf)
					l := runtime.Stack(buf, false)
					log.Error("%v: %s", r, buf[:l])
				} else {
					log.Error("%v", r)
				}
			}
		}()
		
		e.f()
	}()
}
