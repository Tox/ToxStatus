package ping

import (
	"bytes"
	"errors"

	"github.com/alexbakker/tox4go/crypto"
)

type Collection struct {
	List []*Ping
}

func NewCollection() *Collection {
	return &Collection{
		List: []*Ping{},
	}
}

// Find tries to find the given ping ID and public key in the list of pings.
// If 'remove' is set to true, the entry will also be removed from the list.
func (c *Collection) Find(publicKey *[crypto.PublicKeySize]byte, pingID uint64, remove bool) *Ping {
	for i, ping := range c.List {
		if bytes.Equal(ping.PublicKey[:], publicKey[:]) && ping.ID == pingID {
			if remove {
				//remove this pingID from the list
				//doing this here is fine if we break out of the loop right away
				c.removeAt(i)
			}
			return ping
		}
	}

	return nil
}

func (c *Collection) AddNew(publicKey *[crypto.PublicKeySize]byte) (*Ping, error) {
	//remove all expired pings
	c.Clear(true)

	ping, err := NewPing(publicKey)
	if err != nil {
		return nil, err
	}

	err = c.add(ping)
	if err != nil {
		return nil, err
	}

	return ping, nil
}

func (c *Collection) Add(p *Ping) error {
	err := c.add(p)
	if err != nil {
		return err
	}

	return nil
}

func (c *Collection) Clear(expiredOnly bool) {
	for i := len(c.List) - 1; i >= 0; i-- {
		if expiredOnly && !c.List[i].Expired() {
			continue
		}

		c.removeAt(i)
	}
}

func (c *Collection) removeAt(i int) {
	c.List = append(c.List[:i], c.List[i+1:]...)
}

func (c *Collection) add(ping *Ping) error {
	if c.Find(ping.PublicKey, ping.ID, false) != nil {
		return errors.New("item is already in the list")
	}

	c.List = append(c.List, ping)
	return nil
}
