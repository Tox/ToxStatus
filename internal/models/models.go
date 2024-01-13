package models

import (
	"time"

	"github.com/alexbakker/tox4go/dht"
)

type Node struct {
	ID         int64          `json:"-"`
	CreatedAt  time.Time      `json:"created_at"`
	LastSeenAt time.Time      `json:"last_seen_at"`
	PublicKey  *dht.PublicKey `json:"public_key"`
	FQDN       *string        `json:"fqdn"`
	MOTD       *string        `json:"motd"`
	Addresses  []*NodeAddress `json:"addresses"`
}

type NodeAddress struct {
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
