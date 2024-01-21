package models

import (
	"fmt"
	"net"
	"time"

	"github.com/alexbakker/tox4go/dht"
)

type Node struct {
	ID            int64          `json:"-"`
	CreatedAt     time.Time      `json:"created_at"`
	LastSeenAt    time.Time      `json:"last_seen_at"`
	LastInfoReqAt time.Time      `json:"last_info_req_at"`
	LastInfoResAt time.Time      `json:"last_info_res_at"`
	PublicKey     *dht.PublicKey `json:"public_key"`
	FQDN          *string        `json:"fqdn"`
	MOTD          *string        `json:"motd"`
	Version       uint32         `json:"version"`
	Addresses     []*NodeAddress `json:"addresses"`
}

type NodeAddress struct {
	Node       *Node     `json:"-"`
	ID         int64     `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	LastPingAt time.Time `json:"last_ping_at"`
	LastPongAt time.Time `json:"last_pong_at"`
	Net        string    `json:"net"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	Ptr        *string   `json:"ptr"`
}

func (a *NodeAddress) DHTNode() (*dht.Node, error) {
	publicKey := (*dht.PublicKey)(a.Node.PublicKey)

	var nodeType dht.NodeType
	if err := nodeType.UnmarshalText([]byte(a.Net)); err != nil {
		return nil, fmt.Errorf("convert db node: %w", err)
	}

	ip := net.ParseIP(a.IP)
	if ip == nil {
		return nil, fmt.Errorf("bad ip: %s", a.IP)
	}

	return &dht.Node{
		Type:      nodeType,
		PublicKey: publicKey,
		IP:        ip,
		Port:      int(a.Port),
	}, nil
}
