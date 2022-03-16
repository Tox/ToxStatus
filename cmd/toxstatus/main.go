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

	"github.com/alexbakker/tox4go/bootstrap"
	"github.com/alexbakker/tox4go/crypto"
	"github.com/alexbakker/tox4go/dht"
	"github.com/alexbakker/tox4go/dht/ping"
	"github.com/alexbakker/tox4go/relay"
	"github.com/alexbakker/tox4go/transport"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
)

const (
	enableIpv6  = true
	probeRate   = 1 * time.Minute
	refreshRate = 5 * time.Minute
)

var (
	lastScan    int64
	lastRefresh int64
	nodes       = []*toxNode{}
	nodesMutex  = sync.Mutex{}
	tcpPorts    = []int{443, 3389, 33445}
)

func main() {
	if parseFlags() {
		return
	}

	// load state if available
	state, err := loadState()
	if err != nil {
		log.Fatalf("error loading state: %s", err.Error())
	}
	lastScan = state.LastScan
	lastRefresh = state.LastRefresh
	nodes = state.Nodes

	inst, err := NewInstance(":33450")
	if err != nil {
		log.Fatalf("fatal: %s", err.Error())
	}
	inst.UDPTransport.Handle(dht.PacketIDSendNodes, inst.handleSendNodesPacket)
	inst.UDPTransport.Handle(bootstrap.PacketIDBootstrapInfo, handleBootstrapInfoPacket)

	//handle stop signal
	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt)

	//setup http server
	listener, err := net.Listen("tcp4", fmt.Sprintf("localhost:%d", httpListenPort))
	if err != nil {
		log.Fatalf("error in net.Listen: %s", err.Error())
	}
	limiter := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: 2 * time.Second})
	limiter.SetMethods([]string{"POST"})
	limiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", handleHTTPRequest)
	serveMux.Handle("/test", tollbooth.LimitFuncHandler(limiter, handleHTTPRequest))
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
		err := inst.UDPTransport.Listen()
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
	inst.probeNodes()

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
			inst.UDPTransport.Stop()
			//tcpTransport.Stop()
			listener.Close()
			run = false
		case <-probeTicker.C:
			// we want an empty ping list at the start of every probe
			inst.PingsMutex.Lock()
			inst.Pings.Clear(false)
			inst.PingsMutex.Unlock()

			nodesMutex.Lock()
			err := inst.probeNodes()
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
			inst.PingsMutex.Lock()
			inst.Pings.Clear(true)
			inst.PingsMutex.Unlock()

			nodesMutex.Lock()
			for _, node := range nodes {
				if time.Now().Sub(time.Unix(node.LastPing, 0)) > time.Minute*2 {
					node.UDPStatus = false
				}
			}
			sort.Stable(nodeSlice(nodes))

			state := getState()
			saveState(state)
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
		for _, node := range nodes {
			if freshNode.PublicKey == node.PublicKey {
				freshNode.LastPing = node.LastPing
				freshNode.UDPStatus = node.UDPStatus
				freshNode.TCPStatus = node.TCPStatus
				freshNode.TCPPorts = node.TCPPorts
				freshNode.MOTD = node.MOTD
				freshNode.Version = node.Version
				break
			}
		}
	}
	nodes = parsedNodes
	sort.Stable(nodeSlice(nodes))
	nodesMutex.Unlock()

	lastRefresh = time.Now().Unix()
	return nil
}

func (i *instance) probeNodes() error {
	for _, node := range nodes {
		err := i.getBootstrapInfo(node)
		if err != nil {
			fmt.Println(err.Error())
		}

		p, err := i.getNodes(node)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			i.PingsMutex.Lock()
			err = i.Pings.Add(p)
			if err != nil {
				fmt.Println(err.Error())
			}
			i.PingsMutex.Unlock()
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

		go i.probeNodeTCPPorts(node, ports)
	}

	lastScan = time.Now().Unix()
	return nil
}

func (i *instance) probeNodeTCPPorts(node *toxNode, ports []int) {
	c := make(chan int)
	for _, port := range ports {
		go func(p int) {
			conn, err := connectTCP(node, p)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				c <- -1
				return
			}

			err = i.tcpHandshake(node, conn)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				c <- -1
			} else {
				c <- p
			}
			conn.Close()
		}(port)
	}

	newPorts := []int{}
	for i := 0; i < len(ports); i++ {
		port := <-c
		if port != -1 {
			fmt.Printf("tcp port for %s: %d\n", node.Maintainer, port)
			newPorts = append(newPorts, port)
		}
	}

	nodesMutex.Lock()
	node.TCPPorts = newPorts
	if len(node.TCPPorts) > 0 {
		node.LastPing = time.Now().Unix()
	}
	node.TCPStatus = len(node.TCPPorts) > 0
	sort.Stable(nodeSlice(nodes))
	nodesMutex.Unlock()
}

func (i *instance) tcpHandshake(node *toxNode, conn *net.TCPConn) error {
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

	encryptedReqBytes, nonce, err := i.Ident.EncryptBlob(reqBytes, nodePublicKey)
	if err != nil {
		return err
	}

	reqPacket := &relay.HandshakeRequestPacket{
		PublicKey: i.Ident.PublicKey,
		Nonce:     nonce,
		Payload:   encryptedReqBytes,
	}

	reqPacketBytes, err := reqPacket.MarshalBinary()
	if err != nil {
		return err
	}

	conn.SetDeadline(time.Now().Add(2 * time.Second))
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

	decryptedBytes, err := i.Ident.DecryptBlob(res.Payload, nodePublicKey, res.Nonce)
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

func (i *instance) getNodes(node *toxNode) (*ping.Ping, error) {
	nodePublicKey := new([crypto.PublicKeySize]byte)
	decPublicKey, err := hex.DecodeString(node.PublicKey)
	if err != nil {
		return nil, err
	}
	copy(nodePublicKey[:], decPublicKey)

	p, err := ping.NewPing(nodePublicKey)
	if err != nil {
		return nil, err
	}

	packet := &dht.GetNodesPacket{
		PublicKey: i.Ident.PublicKey,
		PingID:    p.ID,
	}

	dhtPacket, err := i.Ident.EncryptPacket(transport.Packet(packet), nodePublicKey)
	if err != nil {
		return nil, err
	}

	payload, err := dhtPacket.MarshalBinary()
	if err != nil {
		return nil, err
	}

	if err := i.sendToUDP(payload, node); err != nil {
		return nil, err
	}

	return p, nil
}

func (i *instance) getBootstrapInfo(node *toxNode) error {
	packet, err := bootstrap.ConstructPacket(&bootstrap.InfoRequestPacket{})
	if err != nil {
		return err
	}

	payload, err := packet.MarshalBinary()
	if err != nil {
		return err
	}

	return i.sendToUDP(payload, node)
}

func (i *instance) sendToUDP(data []byte, node *toxNode) error {
	ip, err := getNodeIP(node)
	if err != nil {
		return err
	}

	return i.UDPTransport.Send(
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
	fmtIp := ""
	if ip.To4() == nil {
		// Wrap IPv6 in brackets
		fmtIp = fmt.Sprintf("[%s]:%d", ip, port)
	} else {
		fmtIp = fmt.Sprintf("%s:%d", ip, port)
	}

	tempConn, err := dialer.Dial("tcp", fmtIp)
	if err != nil {
		return nil, err
	}

	conn, ok := tempConn.(*net.TCPConn)
	if !ok {
		return nil, errors.New("not a tcp conn")
	}

	return conn, nil
}

func (i *instance) handleSendNodesPacket(msg *transport.Message) error {
	dhtPacket := &dht.Packet{}
	err := dhtPacket.UnmarshalBinary(msg.Data)
	if err != nil {
		return err
	}

	decryptedPacket, err := i.Ident.DecryptPacket(dhtPacket)
	if err != nil {
		return err
	}

	packet, ok := decryptedPacket.(*dht.SendNodesPacket)
	if !ok {
		return nil
	}

	i.PingsMutex.Lock()
	ping := i.Pings.Find(dhtPacket.SenderPublicKey, packet.PingID, true)
	i.PingsMutex.Unlock()

	if ping == nil {
		err := fmt.Errorf("sendnodes packet from unknown node: %s:%d", msg.Addr.IP, msg.Addr.Port)
		fmt.Printf("error: %s", err.Error())
		return err
	}

	nodesMutex.Lock()
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
	sort.Stable(nodeSlice(nodes))
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
		if (node.ip4 != nil && node.ip4.Equal(msg.Addr.IP)) ||
			(node.ip6 != nil && node.ip6.Equal(msg.Addr.IP)) {
			node.MOTD = packet.MOTD
			node.Version = fmt.Sprintf("%d", packet.Version)
			break
		}
	}
	nodesMutex.Unlock()

	return nil
}
