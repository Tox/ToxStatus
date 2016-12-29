package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"time"

	"github.com/Impyy/tox4go/dht"
	"github.com/Impyy/tox4go/transport"
)

var (
	networkFlag = flag.String("net", "udp", "network type, either 'udp' or 'tcp'")
	ipFlag      = flag.String("ip", "127.0.0.1", "ip address to probe, ipv4 and ipv6 are both supported")
	portFlag    = flag.Int("port", 33445, "port to probe")
	keyFlag     = flag.String("key", "", "public key of the node")
)

func parseFlags() bool {
	if len(os.Args) < 2 {
		return false
	}

	flag.Parse()

	if len(*keyFlag) != 64 {
		log.Fatalln("error: public key must have a lenght of 64 hex characters")
	}

	ip4, ip6 := resolveIPAddr(*ipFlag, *ipFlag)
	node := toxNode{}
	node.ip4 = ip4.IP
	node.ip6 = ip6.IP
	node.PublicKey = *keyFlag
	node.Port = *portFlag

	if *networkFlag == "udp" {
		err := probeNode(&node)
		if err == nil {
			log.Println("success: this node appears to be online!")
		} else {
			log.Printf("error: %s", err.Error())
			log.Println("fail: this node appears to be offline!")
		}
	} else if *networkFlag == "tcp" {
		err := probeNodeTCP(&node)
		if err == nil {
			log.Println("success: this relay appears to be online!")
		} else {
			log.Printf("error: %s", err.Error())
			log.Println("fail: this relay appears to be offline!")
		}
	} else {
		log.Fatalf("error: unsupported network specified: %s", *networkFlag)
	}

	return true
}

func probeNode(node *toxNode) error {
	resChan := make(chan bool, 1)
	timeoutChan := time.NewTimer(time.Second * 2)

	inst, err := NewInstance("")
	if err != nil {
		return err
	}

	inst.UDPTransport.Handle(dht.PacketIDSendNodes, func(msg *transport.Message) error {
		dhtPacket := &dht.Packet{}
		err := dhtPacket.UnmarshalBinary(msg.Data)
		if err != nil {
			return err
		}

		decryptedPacket, err := inst.Ident.DecryptPacket(dhtPacket)
		if err != nil {
			return err
		}

		packet, ok := decryptedPacket.(*dht.SendNodesPacket)
		if !ok {
			return nil
		}

		resChan <- inst.Pings.Find(dhtPacket.SenderPublicKey, packet.PingID, true) != nil
		return nil
	})
	go inst.UDPTransport.Listen()
	defer inst.UDPTransport.Stop()

	p, err := inst.getNodes(node)
	if err != nil {
		return err
	}
	if err = inst.Pings.Add(p); err != nil {
		return err
	}

	select {
	case success := <-resChan:
		if !success {
			return errors.New("invalid response")
		}
	case <-timeoutChan.C:
		return errors.New("request timed out")
	}

	return nil
}

func probeNodeTCP(node *toxNode) error {
	inst := instance{}
	{
		ident, err := dht.NewIdent()
		if err != nil {
			return err
		}
		inst.Ident = ident
	}

	conn, err := connectTCP(node, node.Port)
	if err != nil {
		return err
	}
	defer conn.Close()

	return inst.tcpHandshake(node, conn)
}
