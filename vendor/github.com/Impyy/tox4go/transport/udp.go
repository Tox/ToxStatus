package transport

import (
	"fmt"
	"net"
)

type UDPTransport struct {
	//note: net.Conn methods can be invoked from multiple goroutines simultaneously.
	conn     *net.UDPConn
	stopChan chan struct{}
	handlers map[byte][]Handler
}

func NewUDPTransport(netProto string, addr string) (*UDPTransport, error) {
	udpAddr, err := net.ResolveUDPAddr(netProto, addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP(netProto, udpAddr)
	if err != nil {
		return nil, err
	}

	return &UDPTransport{
		conn:     conn,
		stopChan: make(chan struct{}),
		handlers: map[byte][]Handler{},
	}, nil
}

func (t *UDPTransport) Send(msg *Message) error {
	_, err := t.conn.WriteTo(msg.Data, msg.Addr)
	return err
}

func (t *UDPTransport) Handle(packetID byte, handler Handler) {
	t.handlers[packetID] = append(t.handlers[packetID], handler)
}

func (t *UDPTransport) Listen() error {
	for {
		buffer := make([]byte, 2048)
		read, sender, err := t.conn.ReadFromUDP(buffer)
		if isNetErrClosing(err) {
			close(t.stopChan)
			return nil
		} else if err != nil {
			fmt.Printf("udp read error: %s", err.Error())
			return err
		}

		if read < 1 {
			continue
		}

		packet := &Message{
			Addr: sender,
			Data: buffer[:read],
		}

		fmt.Printf("received %d from %s:%d\n", packet.Data[0], packet.Addr.IP.String(), packet.Addr.Port)

		handlers, exists := t.handlers[packet.Data[0]]
		if !exists {
			continue
		}

		for _, handler := range handlers {
			/* go */ handler(packet)
		}
	}
}

func (t *UDPTransport) Stop() {
	/*//try the self-pipe trick just in case we're stuck
	addr, ok := t.conn.LocalAddr().(*net.UDPAddr)
	if ok {
		//no need to check for errors here, if sending fails we're fucked anyway
		t.Send(&Message{Data: []byte{}, Addr: addr})
	}*/

	t.conn.Close()
	<-t.stopChan
	//additional cleanup can be done here if needed
}
