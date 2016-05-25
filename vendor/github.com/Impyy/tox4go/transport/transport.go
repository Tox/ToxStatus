package transport

import "net"

type Handler func(packet *Message) error

type Packet interface {
	MarshalBinary() ([]byte, error)
	UnmarshalBinary(data []byte) error
	ID() byte
}

type Transport interface {
	Send(msg *Message) error
	Listen() error
	Stop()
	Handle(packetID byte, handler Handler)
}

type Message struct {
	Data []byte
	Addr *net.UDPAddr
}
