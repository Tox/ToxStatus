package main

import (
	"container/list"
	"strings"
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
		nodes[i] = *node
		i++
	}

	return nodes
}
