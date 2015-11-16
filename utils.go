package main

import (
	"container/list"
	"fmt"
	"strings"
	"time"
	"unicode"
)

func stripSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

//lists can't be marshalled to json objects as easily
func nodesListToSlice(l *list.List) []toxNode {
	nodes := make([]toxNode, nodesList.Len())

	i := 0
	for e := nodesList.Front(); e != nil; e = e.Next() {
		node, _ := e.Value.(*toxNode)
		//while we're at it, let's update the last ping string!
		if node.LastPing != 0 {
			node.LastPingString = fmt.Sprintf("%0.2f min", time.Now().Sub(time.Unix(node.LastPing, 0)).Minutes())
		}
		nodes[i] = *node
		i++
	}

	return nodes
}
