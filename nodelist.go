package gossip

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"net"
)

// NodeList represents a collection of nodes on the network
type NodeList interface {
	GetRandom(count int) []*net.UDPAddr
}

type nodeList struct {
	rand  *rand.Rand
	nodes []*net.UDPAddr
}

// DefaultNodeList returns a default nodelist implementation
func DefaultNodeList(nodes []*net.UDPAddr) NodeList {
	rd := make([]byte, 8)
	crand.Read(rd)

	seed := binary.LittleEndian.Uint64(rd)
	source := rand.NewSource(int64(seed))

	return &nodeList{
		nodes: nodes,
		rand:  rand.New(source),
	}
}

// GetRandom returns a random collection of nodes to gossip to
func (n *nodeList) GetRandom(count int) []*net.UDPAddr {
	ns := make(nodeSet, 0, count)

	if len(n.nodes) < count {
		return n.nodes
	}

	for count > 0 {
		if ns.insert(n.nodes[n.rand.Intn(len(n.nodes))]) {
			count--
		}
	}

	return ns
}

type nodeSet []*net.UDPAddr

func (n *nodeSet) insert(node *net.UDPAddr) bool {
	for i := range *n {
		if (*n)[i].IP.Equal(node.IP) && (*n)[i].Port == node.Port {
			return false
		}
	}

	*n = append(*n, node)

	return true
}
