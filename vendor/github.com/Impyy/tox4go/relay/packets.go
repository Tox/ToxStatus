package relay

import (
	"bytes"

	"github.com/Impyy/tox4go/crypto"
)

type Packet struct {
	Length  uint16
	Payload []byte
}

type HandshakePayload struct {
	PublicKey *[crypto.PublicKeySize]byte
	BaseNonce *[crypto.NonceSize]byte
}

type HandshakeRequestPacket struct {
	PublicKey *[crypto.PublicKeySize]byte
	Nonce     *[crypto.NonceSize]byte
	Payload   []byte
}

type HandshakeResponsePacket struct {
	Nonce   *[crypto.NonceSize]byte
	Payload []byte
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *HandshakePayload) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	_, err := buff.Write(p.PublicKey[:])
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.BaseNonce[:])
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *HandshakePayload) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	p.PublicKey = new([crypto.PublicKeySize]byte)
	_, err := reader.Read(p.PublicKey[:])
	if err != nil {
		return err
	}

	p.BaseNonce = new([crypto.NonceSize]byte)
	_, err = reader.Read(p.BaseNonce[:])
	return err
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *HandshakeRequestPacket) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	_, err := buff.Write(p.PublicKey[:])
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.Nonce[:])
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.Payload[:])
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *HandshakeRequestPacket) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	p.PublicKey = new([crypto.PublicKeySize]byte)
	_, err := reader.Read(p.PublicKey[:])
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
func (p *HandshakeResponsePacket) MarshalBinary() ([]byte, error) {
	buff := new(bytes.Buffer)

	_, err := buff.Write(p.Nonce[:])
	if err != nil {
		return nil, err
	}

	_, err = buff.Write(p.Payload[:])
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryMarshaler interface.
func (p *HandshakeResponsePacket) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)

	p.Nonce = new([crypto.NonceSize]byte)
	_, err := reader.Read(p.Nonce[:])
	if err != nil {
		return err
	}

	p.Payload = make([]byte, reader.Len())
	_, err = reader.Read(p.Payload)
	return err
}
