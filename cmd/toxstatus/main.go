package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"time"

	"github.com/Impyy/tox4go/bootstrap"
	"github.com/Impyy/tox4go/crypto"
	"github.com/Impyy/tox4go/dht"
	"github.com/Impyy/tox4go/dht/ping"
	"github.com/Impyy/tox4go/relay"
	"github.com/Impyy/tox4go/transport"
)

const (
	enableIpv6  = true
	probeRate   = 1 * time.Minute
	refreshRate = 5 * time.Minute
)

var (
	lastScan     int64
	lastRefresh  int64
	udpTransport *transport.UDPTransport
	tcpTransport *transport.TCPTransport
	ident        *dht.Ident
	nodes        = []*toxNode{}
	nodesMutex   = sync.Mutex{}
	pings        = new(ping.Collection)
	pingsMutex   = sync.Mutex{}
	tcpPorts     = []int{443, 3389, 33445}
)

func init() {
	var err error
	ident, err = dht.NewIdent()
	if err != nil {
		panic(err)
	}

	udpTransport, err = transport.NewUDPTransport("udp", ":33450")
	if err != nil {
		panic(err)
	}
	udpTransport.Handle(dht.PacketIDSendNodes, handleSendNodesPacket)
	udpTransport.Handle(bootstrap.PacketIDBootstrapInfo, handleBootstrapInfoPacket)

	/*tcpTransport, err = transport.NewTCPTransport("tcp", ":33450")
	if err != nil {
		panic(err)
	}*/
}

func main() {
	if parseFlags() {
		return
	}

	if err := loadCountries(); err != nil {
		log.Fatalf("error loading countries.json: %s", err.Error())
	}

	//handle stop signal
	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt)

	//setup http server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", httpListenPort))
	if err != nil {
		log.Fatalf("error in net.Listen: %s", err.Error())
	}
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", handleHTTPRequest)
	serveMux.HandleFunc("/json", handleJSONRequest)
	go func() {
		err := http.Serve(listener, serveMux)
		if err != nil {
			log.Printf("http server error: %s\n", err.Error())
			interruptChan <- os.Interrupt
		}
	}()

	//listen for tox packets
	go func() {
		err := udpTransport.Listen()
		if err != nil {
			log.Printf("udp transport error: %s\n", err.Error())
			interruptChan <- os.Interrupt
		}
	}()
	//go tcpTransport.Listen()

	err = refreshNodes()
	if err != nil {
		log.Fatal(err.Error())
	}
	probeNodes()

	probeTicker := time.NewTicker(probeRate)
	refreshTicker := time.NewTicker(refreshRate)
	updateTicker := time.NewTicker(30 * time.Second)
	run := true

	for run {
		select {
		case <-interruptChan:
			fmt.Printf("killing routines\n")
			probeTicker.Stop()
			refreshTicker.Stop()
			updateTicker.Stop()
			udpTransport.Stop()
			//tcpTransport.Stop()
			listener.Close()
			run = false
		case <-probeTicker.C:
			// we want an empty ping list at the start of every probe
			pingsMutex.Lock()
			pings.Clear(false)
			pingsMutex.Unlock()

			nodesMutex.Lock()
			err := probeNodes()
			nodesMutex.Unlock()
			if err != nil {
				log.Printf("error while trying to probe nodes: %s", err.Error())
			}
		case <-refreshTicker.C:
			err := refreshNodes()
			if err != nil {
				log.Printf("error while trying to refresh nodes: %s", err.Error())
			}
		case <-updateTicker.C:
			pingsMutex.Lock()
			pings.Clear(true)
			pingsMutex.Unlock()

			nodesMutex.Lock()
			for _, node := range nodes {
				if time.Now().Sub(time.Unix(node.LastPing, 0)) > time.Minute*2 {
					node.UDPStatus = false
				}
			}
			sort.Stable(nodeSlice(nodes))
			nodesMutex.Unlock()
		}
	}
}

func refreshNodes() error {
	parsedNodes, err := parseNodes()
	if err != nil {
		return err
	}

	nodesMutex.Lock()
	for _, freshNode := range parsedNodes {
		found := false
		for i, node := range nodes {
			if freshNode.PublicKey == node.PublicKey {
				freshNode.LastPing = node.LastPing
				freshNode.UDPStatus = node.UDPStatus
				freshNode.TCPStatus = node.TCPStatus
				freshNode.TCPPorts = node.TCPPorts
				freshNode.MOTD = node.MOTD
				freshNode.Version = node.Version
				nodes[i] = freshNode
				found = true
				break
			}
		}

		if !found {
			nodes = append(nodes, freshNode)
		}
	}
	sort.Stable(nodeSlice(nodes))
	nodesMutex.Unlock()

	lastRefresh = time.Now().Unix()
	return nil
}

func probeNodes() error {
	for _, node := range nodes {
		err := getBootstrapInfo(node)
		if err != nil {
			fmt.Println(err.Error())
		}

		err = getNodes(node)
		if err != nil {
			fmt.Println(err.Error())
		}

		ports := tcpPorts
		exists := false
		for _, i := range ports {
			if i == node.Port {
				exists = true
			}
		}
		if !exists {
			ports = append(ports, node.Port)
		}

		go probeNodeTCPPorts(node, ports)
	}

	lastScan = time.Now().Unix()
	return nil
}

func probeNodeTCPPorts(node *toxNode, ports []int) {
	c := make(chan int)
	for _, port := range ports {
		go func(p int) {
			conn, err := connectTCP(node, p)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				c <- -1
				return
			}

			err = tcpHandshake(node, conn)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				c <- -1
			} else {
				c <- p
			}
			conn.Close()
		}(port)
	}

	nodesMutex.Lock()
	node.TCPPorts = []int{}

	for i := 0; i < len(ports); i++ {
		port := <-c
		if port != -1 {
			fmt.Printf("tcp port for %s: %d\n", node.Maintainer, port)
			node.TCPPorts = append(node.TCPPorts, port)
		}
	}
	if len(node.TCPPorts) > 0 {
		node.LastPing = time.Now().Unix()
	}
	node.TCPStatus = len(node.TCPPorts) > 0
	sort.Stable(nodeSlice(nodes))
	nodesMutex.Unlock()
}

func tcpHandshake(node *toxNode, conn *net.TCPConn) error {
	nodePublicKey := new([crypto.PublicKeySize]byte)
	decPublicKey, err := hex.DecodeString(node.PublicKey)
	if err != nil {
		return err
	}
	copy(nodePublicKey[:], decPublicKey)

	relayConn, err := relay.NewConnection()
	if err != nil {
		return err
	}

	req, err := relayConn.StartHandshake()
	if err != nil {
		return err
	}

	reqBytes, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	encryptedReqBytes, nonce, err := ident.EncryptBlob(reqBytes, nodePublicKey)
	if err != nil {
		return err
	}

	reqPacket := &relay.HandshakeRequestPacket{
		PublicKey: ident.PublicKey,
		Nonce:     nonce,
		Payload:   encryptedReqBytes,
	}

	reqPacketBytes, err := reqPacket.MarshalBinary()
	if err != nil {
		return err
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(reqPacketBytes)
	if err != nil {
		return err
	}

	buffer := make([]byte, 96)
	left := len(buffer)
	for left > 0 {
		read, readErr := conn.Read(buffer[len(buffer)-left:])
		if readErr != nil {
			return readErr
		}
		left -= read
	}

	res := relay.HandshakeResponsePacket{}
	err = res.UnmarshalBinary(buffer)
	if err != nil {
		return err
	}

	decryptedBytes, err := ident.DecryptBlob(res.Payload, nodePublicKey, res.Nonce)
	if err != nil {
		return err
	}

	resPacket := &relay.HandshakePayload{}
	err = resPacket.UnmarshalBinary(decryptedBytes)
	if err != nil {
		return err
	}

	return relayConn.EndHandshake(resPacket)
}

func getNodes(node *toxNode) error {
	nodePublicKey := new([crypto.PublicKeySize]byte)
	decPublicKey, err := hex.DecodeString(node.PublicKey)
	if err != nil {
		return err
	}
	copy(nodePublicKey[:], decPublicKey)

	ping, err := pings.AddNew(nodePublicKey)
	if err != nil {
		return err
	}

	packet := &dht.GetNodesPacket{
		PublicKey: ident.PublicKey,
		PingID:    ping.ID,
	}

	dhtPacket, err := ident.EncryptPacket(transport.Packet(packet), nodePublicKey)
	if err != nil {
		return err
	}

	payload, err := dhtPacket.MarshalBinary()
	if err != nil {
		return err
	}

	return sendToUDP(payload, node)
}

func getBootstrapInfo(node *toxNode) error {
	packet, err := bootstrap.ConstructPacket(&bootstrap.InfoRequestPacket{})
	if err != nil {
		return err
	}

	payload, err := packet.MarshalBinary()
	if err != nil {
		return err
	}

	return sendToUDP(payload, node)
}

func sendToUDP(data []byte, node *toxNode) error {
	ip, err := getNodeIP(node)
	if err != nil {
		return err
	}

	return udpTransport.Send(
		&transport.Message{
			Data: data,
			Addr: &net.UDPAddr{
				IP:   ip,
				Port: node.Port,
			},
		},
	)
}

func getNodeIP(node *toxNode) (net.IP, error) {
	if node.ip4 != nil {
		return node.ip4, nil
	} else if enableIpv6 && node.ip6 != nil {
		return node.ip6, nil
	}

	return nil, fmt.Errorf("no valid ip found for %s", node.Maintainer)
}

func connectTCP(node *toxNode, port int) (*net.TCPConn, error) {
	ip, err := getNodeIP(node)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{}
	dialer.Deadline = time.Now().Add(2 * time.Second)

	tempConn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	conn, ok := tempConn.(*net.TCPConn)
	if !ok {
		return nil, errors.New("not a tcp conn")
	}

	return conn, nil
}

func handleSendNodesPacket(msg *transport.Message) error {
	dhtPacket := &dht.Packet{}
	err := dhtPacket.UnmarshalBinary(msg.Data)
	if err != nil {
		return err
	}

	decryptedPacket, err := ident.DecryptPacket(dhtPacket)
	if err != nil {
		return err
	}

	packet, ok := decryptedPacket.(*dht.SendNodesPacket)
	if !ok {
		return nil
	}

	pingsMutex.Lock()
	nodesMutex.Lock()
	if pings.Find(dhtPacket.SenderPublicKey, packet.PingID, true) != nil {
		for _, node := range nodes {
			publicKey, err := hex.DecodeString(node.PublicKey)
			if err != nil {
				continue
			}

			if bytes.Equal(publicKey, dhtPacket.SenderPublicKey[:]) {
				node.UDPStatus = true
				node.LastPing = time.Now().Unix()
				break
			}
		}
	}
	sort.Stable(nodeSlice(nodes))
	pingsMutex.Unlock()
	nodesMutex.Unlock()

	return nil
}

func handleBootstrapInfoPacket(msg *transport.Message) error {
	bootstrapPacket := &bootstrap.Packet{}
	err := bootstrapPacket.UnmarshalBinary(msg.Data)
	if err != nil {
		return err
	}

	transPacket, err := bootstrap.DestructPacket(bootstrapPacket)
	if err != nil {
		return err
	}

	packet, ok := transPacket.(*bootstrap.InfoResponsePacket)
	if !ok {
		return errors.New("wtf")
	}

	nodesMutex.Lock()
	for _, node := range nodes {
		if node.Ipv4Address == msg.Addr.IP.String() ||
			node.Ipv6Address == msg.Addr.IP.String() {
			node.MOTD = packet.MOTD
			node.Version = fmt.Sprintf("%d", packet.Version)
			break
		}
	}
	nodesMutex.Unlock()

	return nil
}
