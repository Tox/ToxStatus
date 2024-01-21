package crawler

import (
	"encoding/binary"
	"net"

	"github.com/alexbakker/tox4go/dht"
)

func getPublicKeyGenerator(n int) func() *dht.PublicKey {
	var counterBytes [2]byte
	step := (1 << (len(counterBytes) * 8)) / n

	i := 0
	return func() *dht.PublicKey {
		val := step * i
		binary.BigEndian.PutUint16(counterBytes[:], uint16(val))

		i = (i + 1) % n

		var res dht.PublicKey
		copy(res[:len(counterBytes)], counterBytes[:])
		return &res
	}
}

func isGlobalUnicast(ip net.IP) bool {
	return !ip.IsUnspecified() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsMulticast() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast()
}
