package main

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"encoding/json"
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

	"golang.org/x/net/html"
)

const (
	httpListenPort             = 8081
	refreshRate                = 60 //in seconds
	wikiURI                    = "https://wiki.tox.chat/users/nodes?do=edit"
	bootstrapInfoPacketID      = 240
	bootstrapInfoPacketLength  = 78
	bootstrapInfoPacketTimeout = 2 //in seconds
)

var (
	nodesList *list.List
)

type toxNode struct {
	Ipv4Address string `json:"ipv4"`
	Ipv6Address string `json:"ipv6"`
	Port        int    `json:"port"`
	PublicKey   string `json:"public_key"`
	Maintainer  string `json:"maintainer"`
	Location    string `json:"location"`
	Status      bool   `json:"status"`
	Version     string `json:"version"`
	MOTD        string `json:"motd"`
}

func main() {
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
		w.WriteHeader(404)
		w.Write([]byte("404 - " + http.StatusText(404)))
	} else {
		w.Write(data)
	}
}

func renderMainPage(w http.ResponseWriter, urlPath string) {
	tmpl, err2 := template.ParseFiles(path.Join("./assets/", string(urlPath)))

	if err2 != nil {
		w.WriteHeader(500)
		w.Write([]byte("500 - " + http.StatusText(500)))
	} else {
		nodes := make([]toxNode, nodesList.Len())

		i := 0
		for e := nodesList.Front(); e != nil; e = e.Next() {
			node, _ := e.Value.(*toxNode)
			nodes[i] = *node
			i++
		}

		tmpl.Execute(w, nodes)
	}
}

func handleJSONRequest(w http.ResponseWriter, r *http.Request) {
	nodes := make([]toxNode, nodesList.Len())

	i := 0
	for e := nodesList.Front(); e != nil; e = e.Next() {
		node, _ := e.Value.(*toxNode)
		nodes[i] = *node
		i++
	}

	bytes, err := json.Marshal(nodes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal server error: %s", err.Error()), 500)
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

		for e := nodes.Front(); e != nil; e = e.Next() {
			node, _ := e.Value.(*toxNode)
			conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", node.Ipv4Address, node.Port))
			if err != nil {
				continue
			}

			payload := make([]byte, bootstrapInfoPacketLength)
			payload[0] = bootstrapInfoPacketID

			conn.Write(payload)
			conn.SetReadDeadline(time.Now().Add(bootstrapInfoPacketTimeout * time.Second))
			_, err = conn.Read(payload)
			conn.Close()
			if err != nil || payload[0] != bootstrapInfoPacketID {
				continue
			}

			node.Version = fmt.Sprintf("%d", binary.BigEndian.Uint32(payload[1:5]))
			node.MOTD = string(bytes.Trim(payload[5:bootstrapInfoPacketLength], "\x00"))
			node.Status = true
		}

		nodesList = nodes
		time.Sleep(refreshRate * time.Second)
	}
}

func sprintNode(node toxNode) string {
	return fmt.Sprintf("%s:%d in %s by %s", node.Ipv4Address, node.Port, node.Location, node.Maintainer)
}

func probeNode() (motd string, err error) {
	return "", nil
}

func parseNodes() (*list.List, error) {
	res, err := http.Get(wikiURI)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	nodes := list.New()
	tokenizer := html.NewTokenizer(res.Body)

	for {
		token := tokenizer.Next()
		switch token {
		case html.ErrorToken:
			return nil, tokenizer.Err()
		case html.StartTagToken:
			token := tokenizer.Token()
			if token.Data == "textarea" {
				tokenizer.Next()
				token = tokenizer.Token()

				lines := strings.Split(token.Data, "\n")
				for _, line := range lines {
					line = stripSpaces(line)
					if !strings.HasPrefix(line, "|") {
						continue
					}

					lineParts := strings.Split(line, "|")
					if port, err := strconv.Atoi(strings.TrimSpace(lineParts[3])); err == nil && len(lineParts) == 9 {
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
						}
						nodes.PushBack(&node)
					}
				}
				return nodes, nil
			}
		}
	}
}
