package netx

import "io"

type PeerID string
type Addr string

type Conn interface {
	io.ReadWriteCloser
	RemoteAddr() Addr
}

type Network interface {
	Listen(bindAddr string) (listenAddr Addr, err error)
	Accept() (Conn, error)
	Dial(addr Addr) (Conn, error)
	Close() error
}
