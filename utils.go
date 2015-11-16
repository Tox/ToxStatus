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
			node.LastPingString = getSimpleDurationFormat(time.Now().Sub(time.Unix(node.LastPing, 0)))
		}
		nodes[i] = *node
		i++
	}

	return nodes
}

func getSimpleDurationFormat(duration time.Duration) string {
	hours := duration.Hours()

	if hours >= 24 {
		return fmt.Sprintf("%0.0f days", hours/24)
	} else if hours > 1 {
		return fmt.Sprintf("%0.0f hours", hours)
	} else if hours*60 > 1 {
		return fmt.Sprintf("%0.0f minutes", hours*60)
	}

	return fmt.Sprintf("%0.0f seconds", hours*60*60)
}
