package main

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
