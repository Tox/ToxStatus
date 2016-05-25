package ping

import (
	"time"

	"github.com/Impyy/tox4go/crypto"
)

const (
	Timeout = time.Second * 20
)

type Ping struct {
	PublicKey *[crypto.PublicKeySize]byte
	ID        uint64
	Time      time.Time
}

func NewPing(publicKey *[crypto.PublicKeySize]byte) (*Ping, error) {
	pingID, err := crypto.GeneratePingID()
	if err != nil {
		return nil, err
	}

	return &Ping{
		PublicKey: publicKey,
		ID:        pingID,
		Time:      time.Now(),
	}, nil
}

func (p *Ping) Expired() bool {
	return time.Now().Sub(p.Time) > Timeout
}
