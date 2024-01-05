package crawler

import (
	"time"

	"github.com/alexbakker/tox4go/dht"
)

type Node struct {
	*dht.Node
	lastPong        time.Time
	lastPingAttempt time.Time
}

// Pong sets the last pong time of a given node to now.
func (n *Node) Pong() {
	n.lastPong = time.Now()
}
