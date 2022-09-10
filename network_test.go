package gossip

import (
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkGossip(t *testing.T) {
	nodes := make([]*net.UDPAddr, 1000)

	for i := range nodes {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", 10000+i))
		require.Nil(t, err)
		nodes[i] = addr
	}

	nodeList := DefaultNodeList(nodes)

	values := make([][]byte, len(nodes))
	network := make([]*Network, len(nodes))

	var stored int64

	for i := range nodes {
		x := i

		cfg := &Config{
			Nodes:         nodeList,
			Fanout:        DefaultFanout,
			ListenAddress: fmt.Sprintf("127.0.0.1:%d", 10000+i),
			OnGossip: func(message []byte) {
				msg := make([]byte, len(message))
				copy(msg, message)
				values[x] = msg

				atomic.AddInt64(&stored, 1)
			},
		}

		n, err := New(cfg)
		require.Nil(t, err)

		network[i] = n
	}

	err := network[0].Gossip([]byte("hello!"))
	require.Nil(t, err)

	for atomic.LoadInt64(&stored) < int64(len(nodes)) {
		time.Sleep(time.Millisecond)
	}

	fmt.Println("stored", stored)

	for i := range values {
		assert.Equal(t, []byte("hello!"), values[i])
	}
}
