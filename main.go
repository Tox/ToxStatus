package main

import (
	"container/list"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	httpListenPort            = 8081
	refreshRate               = 60 //in seconds
	wikiURI                   = "https://wiki.tox.chat/users/nodes?do=export_raw"
	maxUDPPacketSize          = 2048
	getNodesPacketID          = 2
	sendNodesIpv6PacketID     = 4
	bootstrapInfoPacketID     = 240
	bootstrapInfoPacketLength = 78
	maxMOTDLength             = 256
	queryTimeout              = 4 //in seconds
)

var (
	nodesList = list.New()
	crypto, _ = NewCrypto()
)

type toxNode struct {
	Ipv4Address    string `json:"ipv4"`
	Ipv6Address    string `json:"ipv6"`
	Port           int    `json:"port"`
	PublicKey      string `json:"public_key"`
	Maintainer     string `json:"maintainer"`
	Location       string `json:"location"`
	Status         bool   `json:"status"`
	Version        string `json:"version"`
	MOTD           string `json:"motd"`
	LastPing       int64  `json:"last_ping"`
	LastPingString string `json:"last_ping_string"`
}

func main() {
	if crypto == nil {
		log.Fatalf("Could not generate keypair")
	}

	go probeLoop()

	http.HandleFunc("/", handleHTTPRequest)
	http.HandleFunc("/json", handleJSONRequest)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", httpListenPort), nil))
}

func handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path[1:]
	if r.URL.Path == "/" {
		renderMainPage(w, "index.html")
		return
	}

	//TODO: make this more efficient
	data, err := ioutil.ReadFile(path.Join("./assets/", string(urlPath)))
	if err != nil {
		http.Error(w, http.StatusText(404), 404)
	} else {
		w.Write(data)
	}
}

func renderMainPage(w http.ResponseWriter, urlPath string) {
	tmpl, err2 := template.ParseFiles(path.Join("./assets/", string(urlPath)))

	if err2 != nil {
		http.Error(w, http.StatusText(500), 500)
		log.Printf("Internal server error while trying to serve index: %s", err2.Error())
	} else {
		nodes := nodesListToSlice(nodesList)
		tmpl.Execute(w, nodes)
	}
}

func handleJSONRequest(w http.ResponseWriter, r *http.Request) {
	nodes := nodesListToSlice(nodesList)

	bytes, err := json.Marshal(nodes)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}

	w.Write(bytes)
}

func probeLoop() {
	for {
		nodes, err := parseNodes()
		if err != nil {
			log.Printf("Error while trying to parse nodes: %s", err.Error())
			continue
		}

		c := make(chan *toxNode)
		for e := nodes.Front(); e != nil; e = e.Next() {
			node, _ := e.Value.(*toxNode)
			go func() { c <- probeNode(node) }()
		}

		for i := 0; i < nodes.Len(); i++ {
			_ = <-c
		}

		nodesList = nodes
		time.Sleep(refreshRate * time.Second)
	}
}

func probeNode(node *toxNode) *toxNode {
	conn, err := newNodeConn(node)
	if err != nil {
		return node
	}

	err = getBootstrapInfo(node, conn)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}

	conn.Close()
	conn, err = newNodeConn(node)
	if err != nil {
		return node
	}

	err = getNodes(node, conn)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return node
	}
	conn.Close()

	node.LastPing = time.Now().Unix()
	node.Status = true
	return node
}

func getNodes(node *toxNode, conn net.Conn) error {
	nodePublicKey, err := hex.DecodeString(node.PublicKey)
	if err != nil {
		return err
	}

	plain := make([]byte, len(crypto.PublicKey)+8)
	copy(plain, crypto.PublicKey)
	copy(plain[len(crypto.PublicKey):], nextBytes(8)) //ping id

	nonce := nextNonce()
	sharedKey := crypto.CreateSharedKey(nodePublicKey)
	encrypted := encryptData(plain, sharedKey, nonce)[16:]

	payload := make([]byte, 1+len(crypto.PublicKey)+len(nonce)+len(encrypted))
	payload[0] = getNodesPacketID
	copy(payload[1:], crypto.PublicKey)
	copy(payload[1+len(crypto.PublicKey):], nonce)
	copy(payload[1+len(crypto.PublicKey)+len(nonce):], encrypted)
	conn.Write(payload)

	buffer := make([]byte, maxUDPPacketSize)
	_, err = conn.Read(buffer)

	if err != nil {
		return err
	} /*else if payload[0] != sendNodesIpv6PacketID {
		return fmt.Errorf("packet id: %d is not a sendnodesipv6 packet", payload[0])
	}

	right now we're happy if a node responds to our 'getnodes' request, without even validating the response
	this needs some more work

	on a side note: it looks like nodes are sending a 'getnodes' packet before 'sendnodesipv6',
	*/

	return nil
}

func newNodeConn(node *toxNode) (net.Conn, error) {
	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", node.Ipv4Address, node.Port))
	if err != nil {
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(queryTimeout * time.Second))
	return conn, nil
}

func getBootstrapInfo(node *toxNode, conn net.Conn) error {
	payload := make([]byte, bootstrapInfoPacketLength)
	payload[0] = bootstrapInfoPacketID
	conn.Write(payload)

	buffer := make([]byte, 1+4+maxMOTDLength)
	read, err := conn.Read(buffer)

	if err != nil {
		return err
	} else if buffer[0] != bootstrapInfoPacketID {
		return fmt.Errorf("packet id: %d is not a bootstrap info packet", buffer[0])
	}

	buffer = buffer[:read]
	if len(buffer) < 1+4 {
		return errors.New("bootstrap info packet too small")
	}

	node.Version = fmt.Sprintf("%d", binary.BigEndian.Uint32(buffer[1:1+4]))
	node.MOTD = string(buffer[1+4:])
	return nil
}

func parseNode(nodeString string) *toxNode {
	nodeString = stripSpaces(nodeString)
	if !strings.HasPrefix(nodeString, "|") {
		return nil
	}

	lineParts := strings.Split(nodeString, "|")
	if port, err := strconv.Atoi(strings.TrimSpace(lineParts[3])); err == nil && len(lineParts) == 8 {
		node := toxNode{
			strings.TrimSpace(lineParts[1]),
			strings.TrimSpace(lineParts[2]),
			port,
			strings.TrimSpace(lineParts[4]),
			strings.TrimSpace(lineParts[5]),
			strings.TrimSpace(lineParts[6]),
			false,
			"",
			"",
			0,
			"Never",
		}

		if node.Ipv6Address == "NONE" {
			node.Ipv6Address = "-"
		}

		return &node
	}

	return nil
}

func parseNodes() (*list.List, error) {
	res, err := http.Get(wikiURI)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	nodes := list.New()
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		node := parseNode(line)
		if node == nil {
			continue
		}

		oldNode := getOldNode(node.PublicKey)
		if oldNode != nil { //transfer last ping info
			node.LastPing = oldNode.LastPing
			node.LastPingString = oldNode.LastPingString
		}

		nodes.PushBack(node)
	}
	return nodes, nil
}

func getOldNode(publicKey string) *toxNode {
	for e := nodesList.Front(); e != nil; e = e.Next() {
		node, _ := e.Value.(*toxNode)
		if node.PublicKey == publicKey {
			return node
		}
	}
	return nil
}
