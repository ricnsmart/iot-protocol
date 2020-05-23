package nb

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultMaxBytes = 500 // 字节
	defaultTimeout  = 3 * time.Minute
)

var (
	DeviceOffline      = errors.New("device offline")
	SendMessageTimeout = errors.New("send message timeout")
	WaitMessageTimeout = errors.New("wait message timeout")
)

type (
	Server struct {
		// Addr optionally specifies the TCP address for the server to listen on,
		// in the form "host:port". If empty, ":http" (port 80) is used.
		// The service names are defined in RFC 6335 and assigned by IANA.
		// See net.Dial for details of the address format.
		Addr string

		// 一次性读取字节流的最大长度，默认500个字节
		MaxBytes int

		// 读写超时设置，默认3分钟
		Timeout time.Duration

		// 处理连接
		Handler func(c *Conn)

		// 保存所有活动连接
		activeConn sync.Map

		// 用于调用方执行收尾工作
		AfterConnClose func(id string)

		// 是否打印报文
		debug bool
	}

	// A conn represents the server side of an tcp connection.
	Conn struct {
		// server is the server on which the connection arrived.
		// Immutable; never nil.
		server *Server

		// rwc is the underlying network connection.
		// This is never wrapped by other types and is the value given out
		// to CloseNotifier callers. It is usually of type *net.TCPConn or
		// *tls.Conn.
		rwc net.Conn

		CloseNotifier chan struct{}

		inShutdown int32 // accessed atomically (non-zero means we're in Shutdown)

		// 用于和外界交换数据
		bridgeCh chan []byte

		// 资源读写锁，仅限调用方使用
		sync.Mutex

		// 可供调用方存储一些键值
		sync.Map

		// 可供调用方执行一次性操作
		sync.Once

		// 用于控制写入的频率，防止粘包
		writeSignal chan struct{}

		// 用于标示连接的唯一编号
		id string
	}
)

func NewServer() *Server {
	return &Server{
		MaxBytes: defaultMaxBytes,
		Timeout:  defaultTimeout,
	}
}

func (srv *Server) Debug(debug bool) {
	srv.debug = debug
}

func (srv *Server) StartServer(address string) error {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf(`failed to listen port %v , reason: %v`, address, err)
	}
	defer l.Close()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		rwc, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		c := srv.newConn(rwc)
		srv.activeConn.Store(c, true)
		go srv.Handler(c)
	}
}

// Create new connection from rwc.
func (srv *Server) newConn(rwc net.Conn) *Conn {
	return &Conn{
		server:        srv,
		rwc:           rwc,
		CloseNotifier: make(chan struct{}),
		bridgeCh:      make(chan []byte, 1),
		writeSignal:   make(chan struct{}, 1), // 必须要指定size，否则无法写入
	}
}

func (srv *Server) FindConn(id string) (*Conn, error) {
	c1 := new(Conn)
	srv.activeConn.Range(func(key, value interface{}) bool {
		c := key.(*Conn)
		if c.id == id {
			c1 = c
			return false
		}
		return true
	})
	if c1.id == "" {
		return nil, DeviceOffline
	}
	return c1, nil
}

func (srv *Server) Shutdown() {
	srv.activeConn.Range(func(key, value interface{}) bool {
		key.(*Conn).Close()
		return true
	})
}

func (c *Conn) Read() ([]byte, error) {
	buf := make([]byte, c.server.MaxBytes)
	defer func() {
		if c.server.debug {
			log.Printf(fmt.Sprintf("read:0x% x\n", buf))
		}
	}()
	c.rwc.SetReadDeadline(time.Now().Add(c.server.Timeout))
	readLen, err := c.rwc.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[:readLen]
	return buf, nil
}

func (c *Conn) Write(buf []byte) (n int, err error) {
	// 控制写入频率，防止粘包
	c.writeSignal <- struct{}{}

	defer func() {
		if c.server.debug {
			log.Printf(fmt.Sprintf("write:0x% x\n", buf))
		}
		// 等待1秒之后才允许其他协程使用Write方法
		// 功能和c.Lock相仿，但是c.Lock仅用于调用方使用
		time.Sleep(1 * time.Second)
		<-c.writeSignal
	}()

	c.rwc.SetWriteDeadline(time.Now().Add(c.server.Timeout))
	return c.rwc.Write(buf)
}

func (c *Conn) Send(data []byte) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	select {
	case <-c.CloseNotifier:
		return DeviceOffline
	case c.bridgeCh <- data:
		return nil
	case <-ticker.C:
		return SendMessageTimeout
	}
}

func (c *Conn) Receive() ([]byte, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	select {
	case <-c.CloseNotifier:
		return nil, DeviceOffline
	case buf := <-c.bridgeCh:
		return buf, nil
	case <-ticker.C:
		return nil, WaitMessageTimeout
	}
}

func (c *Conn) Close() {
	if !c.ShuttingDown() {
		atomic.StoreInt32(&c.inShutdown, 1)
		c.server.activeConn.Delete(c)
		close(c.CloseNotifier)
		c.rwc.Close()
		c.server.AfterConnClose(c.id)
	}
}

func (c *Conn) ShuttingDown() bool {
	// TODO: replace inShutdown with the existing atomicBool type;
	// see https://github.com/golang/go/issues/20239#issuecomment-381434582
	return atomic.LoadInt32(&c.inShutdown) != 0
}

// 获取客户端地址
func (c *Conn) RemoteAddr() string {
	return c.rwc.RemoteAddr().String()
}

func (c *Conn) ID() string {
	return c.id
}

func (c *Conn) SetID(id string) {
	// 关闭之前同一设备闲置的连接
	c.server.activeConn.Range(func(key, value interface{}) bool {
		prev := key.(*Conn)
		if prev.id == id {
			prev.Close()
		}
		return false
	})
	c.id = id
}
