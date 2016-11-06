package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type testStatus struct {
	Success bool   `json:"success"`
	Latency int64  `json:"latency"`
	Error   string `json:"error"`
}

type toxStatus struct {
	LastScan    int64      `json:"last_scan"`
	LastRefresh int64      `json:"last_refresh"`
	Nodes       []*toxNode `json:"nodes"`
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
	index       int
	ip4         net.IP
	ip6         net.IP
}

const (
	httpListenPort = 8081
	wikiURI        = "https://wiki.tox.chat/users/nodes?do=export_raw"
)

var (
	countries map[string]string
)

func loadCountries() error {
	const name = "countries.json"
	bytes, ok := assetMap[name]
	if !ok {
		return fmt.Errorf("asset %s not found", name)
	}

	return json.Unmarshal(bytes, &countries)
}

func handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path[1:]
	if r.URL.Path == "/" {
		urlPath = "index"
	}

	if r.Method == "POST" && r.URL.Path == "/test" {
		handleTestRequest(w, r)
		return
	}

	var content interface{}
	var filename string
	switch r.URL.Path {
	case "/":
		content = toxStatus{lastScan, lastRefresh, nodes}
		fallthrough
	case "/test", "/about":
		filename = urlPath + ".html"
	default:
		data, ok := assetMap[urlPath]
		if !ok {
			http.Error(w, http.StatusText(404), 404)
		} else {
			w.Header().Set("Content-Type", mimeTypeByExtension(urlPath))
			w.Write(data)
		}
		return
	}

	err := renderTemplate(w, filename, content)
	if err != nil {
		fmt.Printf("tmpl exec error: %s\n", err.Error())
	}
}

func handleTestRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("test request from: %s\n", r.RemoteAddr)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "unable to parse the form", 500)
		return
	}

	ipPort := r.Form.Get("ipPort")
	key := r.Form.Get("key")
	net := r.Form.Get("net")

	ipPortParts := strings.Split(ipPort, ":")
	if len(ipPortParts) != 2 {
		http.Error(w, "invalid ip:port submission", 500)
		return
	}

	port, err := strconv.Atoi(ipPortParts[1])
	if err != nil {
		http.Error(w, "specified port is not an int", 500)
		return
	}

	ip := ipPortParts[0]
	ip4, ip6 := resolveIPAddr(ip, ip)
	node := toxNode{
		PublicKey: key,
		Port:      port,
	}
	if ip4 != nil {
		node.ip4 = ip4.IP
	}
	if ip6 != nil {
		node.ip6 = ip6.IP
	}

	err = nil
	start := time.Now()
	content := testStatus{Success: true}

	switch net {
	case "UDP":
		err = probeNode(&node)
	case "TCP":
		err = probeNodeTCP(&node)
	default:
		http.Error(w, "invalid net type", 500)
		return
	}

	if err != nil {
		content.Success = false
		content.Error = err.Error()
	} else {
		content.Latency = time.Now().Sub(start).Nanoseconds() / int64(time.Millisecond)
	}

	writeJSONResponse(w, content)
}

func handleJSONRequest(w http.ResponseWriter, r *http.Request) {
	content := toxStatus{lastScan, lastRefresh, nodes}
	writeJSONResponse(w, content)
}

func writeJSONResponse(w http.ResponseWriter, content interface{}) {
	bytes, err := json.Marshal(content)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
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
	for i, line := range lines {
		node := parseNode(line)
		if node == nil {
			continue
		}

		node.index = i
		nodesList = append(nodesList, node)
	}
	return nodesList, nil
}

func parseNode(nodeString string) *toxNode {
	nodeString = stripSpaces(nodeString)
	if !strings.HasPrefix(nodeString, "|") {
		return nil
	}

	lineParts := strings.Split(nodeString, "|")
	var node *toxNode

	if port, err := strconv.Atoi(lineParts[3]); err == nil && len(lineParts) == 8 {
		ip4, ip6 := resolveIPAddr(lineParts[1], lineParts[2])

		node = &toxNode{
			Ipv4Address: lineParts[1],
			Ipv6Address: lineParts[2],
			Port:        port,
			TCPPorts:    []int{},
			PublicKey:   lineParts[4],
			Maintainer:  lineParts[5],
			Location:    lineParts[6],
			UDPStatus:   false,
			TCPStatus:   false,
			Version:     "",
			MOTD:        "",
			LastPing:    0,
			ip4:         nil,
			ip6:         nil,
		}

		if ip4 != nil {
			node.ip4 = ip4.IP
		}
		if ip6 != nil {
			node.ip6 = ip6.IP
		}

		if node.Ipv6Address == "NONE" {
			node.Ipv6Address = "-"
		}
	}

	return node
}

func resolveIPAddr(ip4String string, ip6String string) (*net.IPAddr, *net.IPAddr) {
	ip4, err := net.ResolveIPAddr("ip4", ip4String)
	if err != nil {
		fmt.Printf("couldn't resolve %s: %s\n", ip4String, err.Error())
	}

	ip6, err := net.ResolveIPAddr("ip6", ip6String)
	if err != nil {
		fmt.Printf("couldn't resolve %s: %s\n", ip6String, err.Error())
	}

	return ip4, ip6
}
