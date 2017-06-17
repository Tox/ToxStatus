package bootstrap

/*
This seems like a lot of code for one packet type.
Yes, I know. But I wanted to stay consistent with the rest of the codebase.
*/

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/alexbakker/tox4go/transport"
)

const (
	PacketIDBootstrapInfo   byte = 240
	infoRequestPacketLength      = 78
	maxMOTDLength                = 256
)

// Packet represents the base of all bootstrap node packets.
type Packet struct {
	Type    byte
	Payload []byte
}

// InfoResponsePacket represents the structure of a packet that is sent in
// response to a bootstrap node info request.
type InfoResponsePacket struct {
	Version uint32
	MOTD    string
}

// InfoRequestPacket represents the structure of the packet used to request
// info from a bootstrap node. It contains infoRequestPacketLength - 1 useless
// bytes.
type InfoRequestPacket struct{}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *Packet) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, p.Type)
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.Payload)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *Packet) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	err := binary.Read(reader, binary.BigEndian, &p.Type)
	if err != nil {
		return err
	}

	p.Payload = make([]byte, reader.Len())
	_, err = reader.Read(p.Payload)
	return err
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *InfoResponsePacket) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, p.Version)
	if err != nil {
		return nil, err
	}

	motdBytes := []byte(p.MOTD)
	if len(motdBytes) > maxMOTDLength {
		return nil, errors.New("MOTD too long")
	}

	_, err = buff.Write(motdBytes)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *InfoResponsePacket) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	err := binary.Read(reader, binary.BigEndian, &p.Version)
	if err != nil {
		return err
	}

	motdBytes := make([]byte, reader.Len())
	if len(motdBytes) > maxMOTDLength {
		return errors.New("MOTD too long")
	}

	_, err = reader.Read(motdBytes)
	if err != nil {
		return err
	}

	p.MOTD = string(bytes.Trim(motdBytes, "\x00"))
	return nil
}

// ID returns the packet ID of this packet.
func (p InfoResponsePacket) ID() byte {
	return PacketIDBootstrapInfo
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *InfoRequestPacket) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	zeroes := make([]byte, infoRequestPacketLength-1)
	_, err := buff.Write(zeroes)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), err
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *InfoRequestPacket) UnmarshalBinary(data []byte) error {
	dataLen := len(data)
	expected := infoRequestPacketLength - 1

	if dataLen != expected {
		return fmt.Errorf("invalid packet length: %d, expected: %d", dataLen, expected)
	}

	return nil
}

// ID returns the packet ID of this packet.
func (p InfoRequestPacket) ID() byte {
	return PacketIDBootstrapInfo
}

func DestructPacket(p *Packet) (transport.Packet, error) {
	var tPacket transport.Packet

	switch p.Type {
	case PacketIDBootstrapInfo:
		packetLen := len(p.Payload)

		//horrible, but it's the best we can do
		if packetLen == infoRequestPacketLength-1 && sliceIsZero(p.Payload) {
			tPacket = &InfoRequestPacket{}
		} else {
			tPacket = &InfoResponsePacket{}
		}
	default:
		return nil, fmt.Errorf("unknown packet type: %d", p.Type)
	}

	err := tPacket.UnmarshalBinary(p.Payload)
	if err != nil {
		return nil, err
	}

	return tPacket, nil
}

func ConstructPacket(packet transport.Packet) (*Packet, error) {
	base := &Packet{}
	base.Type = packet.ID()

	payload, err := packet.MarshalBinary()
	if err != nil {
		return nil, err
	}

	base.Payload = payload
	return base, nil
}

func sliceIsZero(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}
