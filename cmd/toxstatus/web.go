package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
)

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
	assetMap = GetAssets()
	funcMap  = template.FuncMap{
		"lower": strings.ToLower,
		"inc":   increment,
		"since": getTimeSinceString,
		"loc":   getLocString,
		"time":  getTimeString,
	}
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
		renderMainPage(w, "index.html")
		return
	}

	data, ok := assetMap[urlPath]
	if !ok {
		http.Error(w, http.StatusText(404), 404)
	} else {
		w.Header().Set("Content-Type", mimeTypeByExtension(urlPath))
		w.Write(data)
	}
}

func renderMainPage(w http.ResponseWriter, urlPath string) {
	data, ok := assetMap["index.html"]
	if !ok {
		http.Error(w, http.StatusText(500), 500)
		return
	}

	tmpl, err := template.New("index.html").Funcs(funcMap).Parse(string(data))
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		log.Printf("Internal server error while trying to serve index: %s", err.Error())
	} else {
		response := toxStatus{lastScan, lastRefresh, nodes}
		tmpl.Execute(w, response)
	}
}

func handleJSONRequest(w http.ResponseWriter, r *http.Request) {
	response := toxStatus{lastScan, lastRefresh, nodes}
	bytes, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}

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
		ip4, err := net.ResolveIPAddr("ip4", lineParts[1])
		if err != nil {
			fmt.Printf("couldn't resolve %s: %s\n", lineParts[1], err.Error())
		}
		ip6, err := net.ResolveIPAddr("ip6", lineParts[2])
		if err != nil {
			fmt.Printf("couldn't resolve %s: %s\n", lineParts[2], err.Error())
		}

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
