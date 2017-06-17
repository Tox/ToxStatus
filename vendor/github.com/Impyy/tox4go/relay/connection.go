package relay

import (
	"errors"

	"github.com/alexbakker/tox4go/crypto"
	"github.com/alexbakker/tox4go/transport"
)

type Connection struct {
	PublicKey     *[crypto.PublicKeySize]byte
	SecretKey     *[crypto.SecretKeySize]byte
	verified      bool
	baseNonce     *[crypto.NonceSize]byte
	peerBaseNonce *[crypto.NonceSize]byte
	peerPublicKey *[crypto.PublicKeySize]byte
}

func NewConnection() (*Connection, error) {
	publicKey, secretKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		PublicKey: publicKey,
		SecretKey: secretKey,
		verified:  false,
	}

	return conn, nil
}

func (c *Connection) StartHandshake() (*HandshakePayload, error) {
	baseNonce, err := crypto.GenerateNonce()
	if err != nil {
		return nil, err
	}
	c.baseNonce = baseNonce

	return &HandshakePayload{
		PublicKey: c.PublicKey,
		BaseNonce: baseNonce,
	}, nil
}

func (c *Connection) EndHandshake(res *HandshakePayload) error {
	c.peerBaseNonce = res.BaseNonce
	c.peerPublicKey = res.PublicKey
	c.verified = true
	return nil
}

func (c *Connection) Verified() bool {
	return c.verified
}

// EncryptPacket encrypts the given packet.
func (c *Connection) EncryptPacket(packet transport.Packet) (*Packet, error) {
	if !c.verified {
		return nil, errors.New("complete a handshake first")
	}

	return nil, errors.New("not implemented")
}

// DecryptPacket decrypts the given packet.
func (c *Connection) DecryptPacket(p *Packet) (transport.Packet, error) {
	if !c.verified {
		return nil, errors.New("complete a handshake first")
	}

	return nil, errors.New("not implemented")
}
