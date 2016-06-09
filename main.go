package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/GoKillers/libsodium-go/cryptobox"
)

const (
	httpListenPort                   = 8081
	refreshRate                      = 60 //in seconds
	wikiURI                          = "https://wiki.tox.chat/users/nodes?do=export_raw"
	maxUDPPacketSize                 = 2048
	getNodesPacketID                 = 2
	sendNodesIpv6PacketID            = 4
	bootstrapInfoPacketID            = 240
	bootstrapInfoPacketLength        = 78
	tcpHandshakePacketLength         = 128
	tcpHandshakeResponsePacketLength = 96
	maxMOTDLength                    = 256
	queryTimeout                     = 4 //in seconds
	dialerTimeout                    = 4 //in seconds
)

var (
	lastScan  int64
	nodes     = []*toxNode{}
	crypto, _ = NewCrypto()
	tcpPorts  = []int{443, 3389, 33445}
	funcMap   = template.FuncMap{
		"lower": strings.ToLower,
		"inc":   increment,
		"since": getTimeSinceString,
		"loc":   getLocString,
		"time":  getTimeString,
	}
	countries map[string]string
)

//flags
var (
	networkFlag = flag.String("net", "udp", "network type, either 'udp' or 'tcp'")
	ipFlag      = flag.String("ip", "127.0.0.1", "ip address to probe, ipv4 and ipv6 are both supported")
	portFlag    = flag.Int("port", 33445, "port to probe")
	keyFlag     = flag.String("key", "", "public key of the node")
)

type tcpHandshakeResult struct {
	Port  int
	Error error
}

type toxStatus struct {
	LastScan int64      `json:"last_scan"`
	Nodes    []*toxNode `json:"nodes"`
}

type toxNode struct {
	Ipv4Address string `json:"ipv4"`
	Ipv6Address string `json:"ipv6"`
	Port        int    `json:"port"`
	TCPPorts    []int  `json:"tcp_ports"`
	PublicKey   string `json:"public_key"`
	Maintainer  string `json:"maintainer"`
	Location    string `json:"location"`
	UDPStatus   bool   `json:"status_udp"`
	TCPStatus   bool   `json:"status_tcp"`
	Version     string `json:"version"`
	MOTD        string `json:"motd"`
	LastPing    int64  `json:"last_ping"`
}

func main() {
	if crypto == nil {
		log.Fatalf("Could not generate keypair")
	}

	if handleFlags() {
		return
	}

	if err := loadCountries(); err != nil {
		log.Fatalf("error loading countries.json: %s", err)
	}

	go probeLoop()

	http.HandleFunc("/", handleHTTPRequest)
	http.HandleFunc("/json", handleJSONRequest)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", httpListenPort), nil))
}

func loadCountries() error {
	bytes, err := ioutil.ReadFile("./assets/countries.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, &countries)
	return err
}

func handleFlags() bool {
	if len(os.Args) < 2 {
		return false
	}

	flag.Parse()

	if len(*keyFlag) != 64 {
		log.Fatalln("error: public key must have a lenght of 64 hex characters")
	}

	node := toxNode{}
	node.Ipv4Address = *ipFlag //HACK: ipv6 addresses will also end up here
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
		w.Header().Set("Content-Type", mimeTypeByExtension(urlPath))
		w.Write(data)
	}
}

func renderMainPage(w http.ResponseWriter, urlPath string) {
	tmpl, err := template.New("index.html").
		Funcs(funcMap).
		ParseFiles(path.Join("./assets/", string(urlPath)))

	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		log.Printf("Internal server error while trying to serve index: %s", err.Error())
	} else {
		response := toxStatus{lastScan, nodes}
		tmpl.Execute(w, response)
	}
}

func handleJSONRequest(w http.ResponseWriter, r *http.Request) {
	response := toxStatus{lastScan, nodes}
	bytes, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}

	w.Write(bytes)
}

func probeLoop() {
	for {
		nodesList, err := parseNodes()
		if err != nil {
			log.Printf("Error while trying to parse nodes: %s", err.Error())
		} else {
			c := make(chan error)
			for _, node := range nodesList {
				go func(node *toxNode) {
					err := probeNode(node)

					ports := tcpPorts
					if !contains(tcpPorts, node.Port) {
						ports = append(ports, node.Port)
					}

					probeNodeTCPPorts(node, ports)

					if node.UDPStatus || node.TCPStatus {
						node.LastPing = time.Now().Unix()
					}

					c <- err
				}(node)
			}

			for _ = range nodesList {
				err = <-c
				if err != nil {
					log.Printf("error: %s", err.Error())
				}
			}

			sort.Stable(nodeSlice(nodesList))
			nodes = nodesList
			lastScan = time.Now().Unix()
		}

		time.Sleep(refreshRate * time.Second)
	}
}

func probeNodeTCPPorts(node *toxNode, ports []int) {
	c := make(chan tcpHandshakeResult)
	for _, port := range ports {
		go func(p int) {
			conn, err := newNodeConn(node, p, "tcp")
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				c <- tcpHandshakeResult{p, err}
			} else {
				c <- tryTCPHandshake(node, conn, p)
			}
		}(port)
	}

	for i := 0; i < len(ports); i++ {
		result := <-c
		if result.Error != nil {
			fmt.Printf("%s\n", result.Error.Error())
		} else {
			node.TCPPorts = append(node.TCPPorts, result.Port)
		}
	}

	node.TCPStatus = len(node.TCPPorts) > 0
}

func probeNodeTCP(node *toxNode) error {
	conn, err := newNodeConn(node, node.Port, "tcp")
	if err != nil {
		return err
	}

	return tryTCPHandshake(node, conn, node.Port).Error
}

func probeNode(node *toxNode) error {
	conn, err := newNodeConn(node, node.Port, "udp")
	if err != nil {
		return err
	}

	err = getBootstrapInfo(node, conn)
	/*if err != nil {
		return err
	}*/
	conn.Close()

	conn, err = newNodeConn(node, node.Port, "udp")
	if err != nil {
		return err
	}

	err = getNodes(node, conn)
	if err != nil {
		conn.Close()
		return err
	}
	conn.Close()

	node.UDPStatus = true
	return nil
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
	node.MOTD = string(bytes.Trim(buffer[1+4:], "\x00"))
	return nil
}

func tryTCPHandshake(node *toxNode, conn net.Conn, port int) tcpHandshakeResult {
	/* NOTE: conn is closed at the end of this function */
	nodePublicKey, err := hex.DecodeString(node.PublicKey)
	if err != nil {
		return tcpHandshakeResult{port, err}
	}

	nonce := nextNonce()
	baseNonce := nextNonce()
	plain := make([]byte, len(crypto.PublicKey)+len(baseNonce))
	tempCrypto, _ := NewCrypto()

	copy(plain, tempCrypto.PublicKey)
	copy(plain[len(tempCrypto.PublicKey):], baseNonce)
	sharedKey := crypto.CreateSharedKey(nodePublicKey)
	encrypted := encryptData(plain, sharedKey, nonce)[16:]

	payload := make([]byte, tcpHandshakePacketLength)
	copy(payload, crypto.PublicKey)
	copy(payload[len(crypto.PublicKey):], nonce)
	copy(payload[len(crypto.PublicKey)+len(nonce):], encrypted)
	conn.Write(payload)

	buffer := make([]byte, tcpHandshakeResponsePacketLength)
	read, err := conn.Read(buffer)

	var result tcpHandshakeResult

	if err != nil {
		result = tcpHandshakeResult{port, err}
	} else if read != tcpHandshakeResponsePacketLength {
		result = tcpHandshakeResult{
			port,
			errors.New("tcp handshake response had an invalid length"),
		}
	} else if isValidHandshakeResponse(buffer, baseNonce, sharedKey, tempCrypto) {
		result = tcpHandshakeResult{
			port,
			errors.New("tcp handshake response is incorrect"),
		}
	} else {
		result = tcpHandshakeResult{port, nil}
	}

	conn.Close()
	return result
}

func isValidHandshakeResponse(data []byte, baseNonce []byte, sharedKey []byte, tempPair *Crypto) bool {
	nonceSize := cryptobox.CryptoBoxNonceBytes()
	nonce := data[:nonceSize]
	encrypted := data[nonceSize:]

	decrypted := decryptData(encrypted, sharedKey, nonce)
	if decrypted == nil {
		return false
	}

	serverBaseNonce := decrypted[:nonceSize]
	tempPublicKey := decrypted[nonceSize:]

	if !bytes.Equal(tempPair.PublicKey, tempPublicKey) {
		return false
	}

	if !bytes.Equal(baseNonce, serverBaseNonce) {
		return false
	}

	return true
}

func newNodeConn(node *toxNode, port int, network string) (net.Conn, error) {
	dialer := net.Dialer{}
	dialer.Deadline = time.Now().Add(dialerTimeout * time.Second)

	conn, err := dialer.Dial(network, fmt.Sprintf("%s:%d", node.Ipv4Address, port))
	if err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(queryTimeout * time.Second))
	return conn, nil
}

func parseNode(nodeString string) *toxNode {
	nodeString = stripSpaces(nodeString)
	if !strings.HasPrefix(nodeString, "|") {
		return nil
	}

	lineParts := strings.Split(nodeString, "|")
	if port, err := strconv.Atoi(lineParts[3]); err == nil && len(lineParts) == 8 {
		node := toxNode{
			lineParts[1],
			lineParts[2],
			port,
			[]int{},
			lineParts[4],
			lineParts[5],
			lineParts[6],
			false,
			false,
			"",
			"",
			0,
		}

		if node.Ipv6Address == "NONE" {
			node.Ipv6Address = "-"
		}

		return &node
	}

	return nil
}

func parseNodes() ([]*toxNode, error) {
	res, err := http.Get(wikiURI)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	nodesList := []*toxNode{}
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
		}

		nodesList = append(nodesList, node)
	}
	return nodesList, nil
}

func getOldNode(publicKey string) *toxNode {
	for _, node := range nodes {
		if node.PublicKey == publicKey {
			return node
		}
	}
	return nil
}

type nodeSlice []*toxNode

func (c nodeSlice) Len() int {
	return len(c)
}

func (c nodeSlice) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c nodeSlice) Less(i, j int) bool {
	if c[i].UDPStatus != c[j].UDPStatus {
		return c[i].UDPStatus
	}

	if c[i].TCPStatus != c[j].TCPStatus {
		return c[i].TCPStatus
	}

	return false
}
