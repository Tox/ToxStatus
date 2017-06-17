package bootstrap

import (
	"fmt"
	"net"

	"github.com/alexbakker/tox4go/crypto"
	"github.com/alexbakker/tox4go/dht"
	"github.com/alexbakker/tox4go/dht/ping"
	"github.com/alexbakker/tox4go/transport"
)

type Node struct {
	Ident       *dht.Ident
	Info        *NodeInfo
	IsBootstrap bool
	tp          transport.Transport
	pings       *ping.Collection
}

type NodeInfo struct {
	Version uint32
	MOTD    string
}

func NewNode(tp transport.Transport) (*Node, error) {
	ident, err := dht.NewIdent()
	if err != nil {
		return nil, err
	}

	node := &Node{
		Ident: ident,
		Info: &NodeInfo{
			Version: 1234,
			MOTD:    "tox4go bootstrap node",
		},
		IsBootstrap: false,
		tp:          tp,
		pings:       new(ping.Collection),
	}

	tp.Handle(dht.PacketIDPingRequest, node.handleDHTPacket)
	tp.Handle(dht.PacketIDPingResponse, node.handleDHTPacket)
	tp.Handle(dht.PacketIDGetNodes, node.handleDHTPacket)
	tp.Handle(dht.PacketIDSendNodes, node.handleDHTPacket)
	tp.Handle(PacketIDBootstrapInfo, node.handleBootstrapPacket)

	return node, nil
}

func (n *Node) Bootstrap(node *dht.Node) error {
	return n.Query(node, n.Ident.PublicKey)
}

// Query queries the given DHT node to search for the given publicKey.
func (n *Node) Query(node *dht.Node, publicKey *[crypto.PublicKeySize]byte) error {
	ping, err := n.pings.AddNew(node.PublicKey)
	if err != nil {
		return err
	}

	packet := &dht.GetNodesPacket{
		PublicKey: publicKey,
		PingID:    ping.ID,
	}

	return n.sendPacket(packet, node)
}

func (n *Node) Ping(node *dht.Node) error {
	ping, err := n.pings.AddNew(node.PublicKey)
	if err != nil {
		return err
	}

	packet := &dht.PingRequestPacket{
		PingID: ping.ID,
	}

	return n.sendPacket(packet, node)
}

func (n *Node) Pings() *ping.Collection {
	return n.pings
}

func (n *Node) sendPacket(packet transport.Packet, destNode *dht.Node) error {
	dhtPacket, err := n.Ident.EncryptPacket(packet, destNode.PublicKey)
	if err != nil {
		return err
	}

	packetBytes, err := dhtPacket.MarshalBinary()
	if err != nil {
		return err
	}

	return n.tp.Send(&transport.Message{
		Data: packetBytes,
		Addr: &net.UDPAddr{
			IP:   destNode.IP,
			Port: destNode.Port,
		},
	})
}

func (n *Node) handleDHTPacket(msg *transport.Message) error {
	dhtPacket := &dht.Packet{}
	err := dhtPacket.UnmarshalBinary(msg.Data)
	if err != nil {
		return err
	}

	decryptedPacket, err := n.Ident.DecryptPacket(dhtPacket)
	if err != nil {
		return err
	}

	node := &dht.Node{
		IP:        msg.Addr.IP,
		Port:      msg.Addr.Port,
		PublicKey: dhtPacket.SenderPublicKey,
		Type:      dht.NodeTypeUDP,
	}

	switch packet := decryptedPacket.(type) {
	case *dht.GetNodesPacket:
		err = n.handleGetNodesPacket(node, packet)
	case *dht.SendNodesPacket:
		err = n.handleSendNodesPacket(node, packet)
	case *dht.PingRequestPacket:
		err = n.handlePingRequestPacket(node, packet)
	case *dht.PingResponsePacket:
		err = n.handlePingResponsePacket(node, packet)
	default:
		err = fmt.Errorf("unknown packet type: %d", packet.ID())
	}

	return err
}

func (n *Node) handleGetNodesPacket(node *dht.Node, packet *dht.GetNodesPacket) error {
	return nil
}

func (n *Node) handleSendNodesPacket(node *dht.Node, packet *dht.SendNodesPacket) error {
	ping := n.pings.Find(node.PublicKey, packet.PingID, true)
	if ping != nil {
		//this is where we'd update the last ping time of a dht friend
	} else {
		//who is this?
	}

	for _, node := range packet.Nodes {
		//only call bootstrap if we don't know this friend yet
		n.Bootstrap(node)
	}
	return nil
}

func (n *Node) handlePingRequestPacket(node *dht.Node, packet *dht.PingRequestPacket) error {
	res := &dht.PingResponsePacket{PingID: packet.PingID}
	return n.sendPacket(res, node)
}

func (n *Node) handlePingResponsePacket(node *dht.Node, packet *dht.PingResponsePacket) error {
	ping := n.pings.Find(node.PublicKey, packet.PingID, true)
	if ping != nil {
		//this is where we'd update the last ping time of a dht friend
	} else {
		//who is this?
	}
	return nil
}

func (n *Node) handleBootstrapPacket(msg *transport.Message) error {
	if !n.IsBootstrap {
		return nil
	}

	packet := &Packet{}
	err := packet.UnmarshalBinary(msg.Data)
	if err != nil {
		return err
	}

	innerPacket, err := DestructPacket(packet)
	if err != nil {
		return err
	}

	switch innerPacket.(type) {
	case *InfoRequestPacket:
		res := &InfoResponsePacket{Version: n.Info.Version, MOTD: n.Info.MOTD}
		resPacket, err := ConstructPacket(res)
		if err != nil {
			return err
		}

		packetBytes, err := resPacket.MarshalBinary()
		if err != nil {
			return err
		}

		return n.tp.Send(&transport.Message{Data: packetBytes, Addr: msg.Addr})
	default:
		return nil
	}
}
