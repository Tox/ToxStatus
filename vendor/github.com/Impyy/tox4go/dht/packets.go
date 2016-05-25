package dht

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/Impyy/tox4go/crypto"
)

const (
	PacketIDPingRequest  byte = 0
	PacketIDPingResponse byte = 1
	PacketIDGetNodes     byte = 2
	PacketIDSendNodes    byte = 4

	nodeTypeUDPIpv4 byte = 2
	nodeTypeUDPIpv6 byte = 10
	nodeTypeTCPIpv4 byte = 130
	nodeTypeTCPIpv6 byte = 138
)

// NodeType represents the type of Node.
// This can be a regular node (UDP) or a TCP relay.
type NodeType byte

const (
	// NodeTypeUDP represents the UDP node type.
	NodeTypeUDP NodeType = iota
	// NodeTypeTCP represents the TCP node type.
	NodeTypeTCP
)

// Packet represents the base of all DHT packets.
type Packet struct {
	Type            byte
	SenderPublicKey *[crypto.PublicKeySize]byte
	Nonce           *[crypto.NonceSize]byte
	Payload         []byte /* encrypted */
}

// Node represents a node in the DHT.
type Node struct {
	Type      NodeType
	PublicKey *[crypto.PublicKeySize]byte
	IP        net.IP
	Port      int
}

// GetNodesPacket represents the encrypted portion of the GetNodes request.
type GetNodesPacket struct {
	PublicKey *[crypto.PublicKeySize]byte
	PingID    uint64
}

// SendNodesPacket represents the encrypted portion of the SendNodes packet.
type SendNodesPacket struct {
	Nodes  []*Node
	PingID uint64
}

// PingResponsePacket represents the encrypted portion of the PingResponse packet.
type PingResponsePacket struct {
	//ping type: 0x01
	PingID uint64
}

// PingRequestPacket represents the encrypted portion of the PingRequest packet.
type PingRequestPacket struct {
	//ping type: 0x00
	PingID uint64
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *GetNodesPacket) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	_, err := buff.Write(p.PublicKey[:])
	if err != nil {
		return nil, err
	}

	err = binary.Write(buff, binary.BigEndian, p.PingID)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *GetNodesPacket) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	p.PublicKey = new([crypto.PublicKeySize]byte)
	_, err := reader.Read(p.PublicKey[:])
	if err != nil {
		return err
	}

	return binary.Read(reader, binary.BigEndian, &p.PingID)
}

// ID returns the packet ID of this packet.
func (p GetNodesPacket) ID() byte {
	return PacketIDGetNodes
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *Packet) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, p.Type)
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.SenderPublicKey[:])
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.Nonce[:])
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.Payload)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *Packet) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	err := binary.Read(reader, binary.BigEndian, &p.Type)
	if err != nil {
		return err
	}

	p.SenderPublicKey = new([crypto.PublicKeySize]byte)
	_, err = reader.Read(p.SenderPublicKey[:])
	if err != nil {
		return err
	}

	p.Nonce = new([crypto.NonceSize]byte)
	_, err = reader.Read(p.Nonce[:])
	if err != nil {
		return err
	}

	p.Payload = make([]byte, reader.Len())
	_, err = reader.Read(p.Payload)
	return err
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *SendNodesPacket) MarshalBinary() ([]byte, error) {
	if len(p.Nodes) > 4 {
		return nil, errors.New("too many nodes, the max is 4")
	}

	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, byte(len(p.Nodes)))
	if err != nil {
		return nil, err
	}

	for _, node := range p.Nodes {
		nodeBytes, err2 := node.MarshalBinary()
		if err2 != nil {
			return nil, err2
		}

		_, err2 = buff.Write(nodeBytes)
		if err2 != nil {
			return nil, err2
		}
	}

	err = binary.Write(buff, binary.BigEndian, p.PingID)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *SendNodesPacket) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	var count byte
	err := binary.Read(reader, binary.BigEndian, &count)
	if err != nil {
		return err
	}

	if count > 4 {
		return errors.New("too many nodes, the max is 4")
	}

	p.Nodes = make([]*Node, int(count))
	for i := range p.Nodes {
		var ipType byte
		var ipSize int

		p.Nodes[i] = &Node{}

		err = binary.Read(reader, binary.BigEndian, &ipType)
		if err != nil {
			return err
		}

		switch ipType {
		case nodeTypeUDPIpv4, nodeTypeTCPIpv4: //ipv4
			ipSize = net.IPv4len
		case nodeTypeUDPIpv6, nodeTypeTCPIpv6: //ipv6
			ipSize = net.IPv6len
		default:
			return fmt.Errorf("unknown address family: %d", ipType)
		}

		nodeBytes := make([]byte, 1+ipSize+2+crypto.PublicKeySize)
		nodeBytes[0] = ipType
		_, err := reader.Read(nodeBytes[1:])
		if err != nil {
			return err
		}

		err = p.Nodes[i].UnmarshalBinary(nodeBytes)
		if err != nil {
			return err
		}
	}

	return binary.Read(reader, binary.BigEndian, &p.PingID)
}

// ID returns the packet ID of this packet.
func (p SendNodesPacket) ID() byte {
	return PacketIDSendNodes
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *PingResponsePacket) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, PacketIDPingResponse)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buff, binary.BigEndian, p.PingID)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *PingResponsePacket) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	var pingType byte
	err := binary.Read(reader, binary.BigEndian, &pingType)
	if err != nil {
		return err
	} else if pingType != PacketIDPingResponse {
		return fmt.Errorf("incorrect ping type: %d! is this a replay attack?", pingType)
	}

	return binary.Read(reader, binary.BigEndian, &p.PingID)
}

// ID returns the packet ID of this packet.
func (p PingResponsePacket) ID() byte {
	return PacketIDPingResponse
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *PingRequestPacket) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, PacketIDPingRequest)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buff, binary.BigEndian, p.PingID)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *PingRequestPacket) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	var pingType byte
	err := binary.Read(reader, binary.BigEndian, &pingType)
	if err != nil {
		return err
	} else if pingType != PacketIDPingRequest {
		return fmt.Errorf("incorrect ping type: %d! is this a replay attack?", pingType)
	}

	return binary.Read(reader, binary.BigEndian, &p.PingID)
}

// ID returns the packet ID of this packet.
func (p PingRequestPacket) ID() byte {
	return PacketIDPingRequest
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (n *Node) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)
	var ipType byte
	var ipSize int

	err := binary.Read(reader, binary.BigEndian, &ipType)
	if err != nil {
		return err
	}

	switch ipType {
	case nodeTypeUDPIpv4:
		ipSize = net.IPv4len
		n.Type = NodeTypeUDP
	case nodeTypeTCPIpv4:
		ipSize = net.IPv4len
		n.Type = NodeTypeTCP
	case nodeTypeUDPIpv6:
		ipSize = net.IPv6len
		n.Type = NodeTypeUDP
	case nodeTypeTCPIpv6:
		ipSize = net.IPv6len
		n.Type = NodeTypeTCP
	default:
		return fmt.Errorf("unknown address family: %d", ipType)
	}

	n.IP = make([]byte, ipSize)
	_, err = reader.Read(n.IP)
	if err != nil {
		return err
	}

	var port uint16
	err = binary.Read(reader, binary.BigEndian, &port)
	if err != nil {
		return err
	}
	n.Port = int(port)

	n.PublicKey = new([crypto.PublicKeySize]byte)
	_, err = reader.Read(n.PublicKey[:])
	return err
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (n *Node) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	//To4 returns nil if n.Addr.IP is not an IPv4 address
	ipv4 := n.IP.To4()
	var err error

	var addrFam byte
	if ipv4 != nil && n.Type == NodeTypeUDP {
		addrFam = nodeTypeUDPIpv4
	} else if ipv4 != nil && n.Type == NodeTypeTCP {
		addrFam = nodeTypeTCPIpv4
	} else if ipv4 == nil && n.Type == NodeTypeUDP {
		addrFam = nodeTypeUDPIpv6
	} else if ipv4 == nil && n.Type == NodeTypeTCP {
		addrFam = nodeTypeTCPIpv6
	} else {
		return nil, errors.New("unable to determine address family")
	}

	err = binary.Write(buff, binary.BigEndian, addrFam)
	if err != nil {
		return nil, err
	}

	if ipv4 != nil {
		_, err = buff.Write(ipv4)
	} else {
		_, err = buff.Write(n.IP)
	}

	if err != nil {
		return nil, err
	}

	err = binary.Write(buff, binary.BigEndian, uint16(n.Port))
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(n.PublicKey[:])
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}
