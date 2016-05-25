package transport

import "net"

type TCPTransport struct {
	//note: net.Listener methods can be invoked from multiple goroutines simultaneously.
	listener *net.TCPListener
	stopChan chan struct{}
	handlers map[byte][]Handler
}

func NewTCPTransport(netProto string, addr string) (*TCPTransport, error) {
	tcpAddr, err := net.ResolveTCPAddr(netProto, addr)
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenTCP(netProto, tcpAddr)
	if err != nil {
		return nil, err
	}

	return &TCPTransport{
		listener: listener,
		stopChan: make(chan struct{}),
		handlers: map[byte][]Handler{},
	}, nil
}
