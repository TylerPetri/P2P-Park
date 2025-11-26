package netx

import (
	"net"
	"sync"
)

type tcpNetwork struct {
	mu       sync.Mutex
	listener net.Listener
}

func NewTCPNetwork() Network {
	return &tcpNetwork{}
}

func (t *tcpNetwork) Listen(bindAddr string) (Addr, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	l, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return "", err
	}
	t.listener = l
	return Addr(l.Addr().String()), nil
}

func (t *tcpNetwork) Accept() (Conn, error) {
	t.mu.Lock()
	l := t.listener
	t.mu.Unlock()

	if l == nil {
		return nil, net.ErrClosed
	}
	c, err := l.Accept()
	if err != nil {
		return nil, err
	}
	return &tcpConn{Conn: c}, nil
}

func (t *tcpNetwork) Dial(addr Addr) (Conn, error) {
	c, err := net.Dial("tcp", string(addr))
	if err != nil {
		return nil, err
	}
	return &tcpConn{Conn: c}, nil
}

func (t *tcpNetwork) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.listener != nil {
		err := t.listener.Close()
		t.listener = nil
		return err
	}
	return nil
}

type tcpConn struct {
	net.Conn
}

func (c *tcpConn) RemoteAddr() Addr {
	return Addr(c.Conn.RemoteAddr().String())
}
