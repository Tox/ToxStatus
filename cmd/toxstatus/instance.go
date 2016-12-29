package main

import (
	"fmt"
	"sync"

	"github.com/Impyy/tox4go/dht"
	"github.com/Impyy/tox4go/dht/ping"
	"github.com/Impyy/tox4go/transport"
)

type instance struct {
	UDPTransport *transport.UDPTransport
	TCPTransport *transport.TCPTransport
	Ident        *dht.Ident
	Pings        *ping.Collection
	PingsMutex   *sync.Mutex
}

func NewInstance(udpAddr string) (*instance, error) {
	inst := instance{
		Pings:      new(ping.Collection),
		PingsMutex: new(sync.Mutex),
	}

	var err error
	inst.Ident, err = dht.NewIdent()
	if err != nil {
		return nil, fmt.Errorf("error creating new dht identity: %s", err)
	}

	inst.UDPTransport, err = transport.NewUDPTransport("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("error creating new udp transport instance: %s", err)
	}

	return &inst, nil
}
