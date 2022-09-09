package gossip

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/maphash"
	"log"
	"net"
	"sync"
	"syscall"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/purehyperbole/gossip/protocol"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

// DefaultFanout the default fanout factor 'k'
const DefaultFanout = 13

// Network defines the network that can be gossiped to/from
type Network struct {
	id         []byte
	fanout     int
	nodes      NodeList
	callback   func(message []byte)
	seed       maphash.Seed
	timer      *time.Ticker
	tracker    sync.Map
	conn       *ipv4.PacketConn
	readBatch  []ipv4.Message
	writeBatch []ipv4.Message
	builder    sync.Pool
	mu         sync.Mutex
}

// New creates a new listener on the gossip network
func New(cfg *Config) (*Network, error) {
	if cfg.Fanout < 1 {
		cfg.Fanout = DefaultFanout
	}

	lcfg := net.ListenConfig{
		Control: control,
	}

	// start one of several listeners
	c, err := lcfg.ListenPacket(context.Background(), "udp", cfg.ListenAddress)
	if err != nil {
		return nil, err
	}

	addr := c.LocalAddr().(*net.UDPAddr)

	id := make([]byte, 6)
	copy(id, addr.IP)
	binary.LittleEndian.PutUint16(id[4:], uint16(addr.Port))

	n := &Network{
		id:         id,
		fanout:     cfg.Fanout,
		nodes:      cfg.Nodes,
		callback:   cfg.OnGossip,
		seed:       maphash.MakeSeed(),
		timer:      time.NewTicker(time.Millisecond),
		conn:       ipv4.NewPacketConn(c),
		readBatch:  make([]ipv4.Message, 1024),
		writeBatch: make([]ipv4.Message, 1024),
		builder: sync.Pool{
			New: func() any {
				return flatbuffers.NewBuilder(1024)
			},
		},
	}

	for i := 0; i < 1024; i++ {
		n.readBatch[i].Buffers = [][]byte{make([]byte, 1500)}
		n.writeBatch[i].Buffers = [][]byte{make([]byte, 1500)}
	}

	// reset the len on the write batch
	n.writeBatch = n.writeBatch[:0]

	go n.listen()
	go n.cleanup()
	go n.flusher()

	return n, nil
}

// Gossip gossip a network to the
func (n *Network) Gossip(message []byte) error {
	return n.gossip(n.id, message)
}

func (n *Network) gossip(origin, message []byte) error {
	nodes := n.nodes.GetRandom(n.fanout)

	b := n.builder.Get().(*flatbuffers.Builder)
	defer n.builder.Put(b)

	sb := b.CreateByteVector(origin)
	mb := b.CreateByteVector(message)

	protocol.EventStart(b)
	protocol.EventAddOrigin(b, sb)
	protocol.EventAddMessage(b, mb)
	ev := protocol.EventEnd(b)

	b.Finish(ev)
	fb := b.FinishedBytes()

	n.mu.Lock()
	defer n.mu.Unlock()

	for i := range nodes {
		x := len(n.writeBatch)

		if x == cap(n.writeBatch) {
			err := n.flush(false)
			if err != nil {
				return err
			}
		}

		n.writeBatch = n.writeBatch[:len(n.writeBatch)+1]
		n.writeBatch[x].Addr = nodes[i]
		n.writeBatch[x].Buffers[0] = n.writeBatch[x].Buffers[0][:len(fb)]
		copy(n.writeBatch[x].Buffers[0], fb)
	}

	return nil
}

func (n *Network) listen() {
	var hasher maphash.Hash
	hasher.SetSeed(n.seed)

	for {
		rb, err := n.conn.ReadBatch(n.readBatch, 0)
		if err != nil {
			log.Fatalf("socket read failed: %s", err.Error())
		}

		now := time.Now()

		for i := 0; i < rb; i++ {
			sm := n.readBatch[i]

			data := sm.Buffers[0][:sm.N]
			event := protocol.GetRootAsEvent(data, 0)

			hasher.Reset()
			hasher.Write(data)
			key := hasher.Sum64()

			_, seen := n.tracker.Load(key)
			if seen {
				continue
			}

			n.tracker.Store(key, now)

			n.callback(event.MessageBytes())

			err = n.gossip(event.OriginBytes(), event.MessageBytes())
			if err != nil {
				log.Fatalf("failed to gossip message: %s", err.Error())
			}
		}
	}
}

func (n *Network) cleanup() {
	for {
		time.Sleep(time.Second * 10)

		threshold := time.Now().Add(-time.Minute * 5)

		n.tracker.Range(func(key, value interface{}) bool {
			if value.(time.Time).Before(threshold) {
				n.tracker.Delete(key)
			}
			return true
		})
	}
}

func (n *Network) flusher() {
	for {
		_, ok := <-n.timer.C
		if !ok {
			return
		}

		err := n.flush(true)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Fatalf("write flush failed: %s", err.Error())
		}
	}
}

func (n *Network) flush(lock bool) error {
	if lock {
		n.mu.Lock()
		defer n.mu.Unlock()
	}

	if len(n.writeBatch) < 1 {
		return nil
	}

	_, err := n.conn.WriteBatch(n.writeBatch, 0)
	if err != nil {
		return err
	}

	// reset the batch
	n.writeBatch = n.writeBatch[:0]

	return nil
}

// "borrow" this from github.com/libp2p/go-reuseport as we don't care about other operating systems right now :)
func control(network, address string, c syscall.RawConn) error {
	var err error

	c.Control(func(fd uintptr) {
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if err != nil {
			return
		}

		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		if err != nil {
			return
		}
	})

	return err
}
