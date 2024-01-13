package db

import (
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/alexbakker/tox4go/dht"
)

type Time time.Time

// Scan implements the sql.Scanner interface.
func (t *Time) Scan(src any) error {
	if src == nil {
		*t = Time{}
		return nil
	}

	f, ok := src.(float64)
	if !ok {
		return fmt.Errorf("can't scan into db.Time: %T", src)
	}

	*t = Time(time.UnixMilli(int64(f * 1000)))
	return nil
}

type PublicKey dht.PublicKey

// Scan implements the sql.Scanner interface.
func (k *PublicKey) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("can't scan into db.PublicKey: %T", src)
	}

	ds, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	*k = (PublicKey)(ds)
	return nil
}

// Value implements the driver.Valuer interface.
func (k *PublicKey) Value() (driver.Value, error) {
	return hex.EncodeToString(k[:]), nil
}
