# gossip [![Go Reference](https://pkg.go.dev/badge/github.com/purehyperbole/gossip.svg)](https://pkg.go.dev/github.com/purehyperbole/gossip) [![Go Report Card](https://goreportcard.com/badge/github.com/purehyperbole/gossip)](https://goreportcard.com/report/github.com/purehyperbole/gossip) ![Build Status](https://github.com/purehyperbole/gossip/actions/workflows/ci.yml/badge.svg)

A simple gossip protocol implementation in < 200 lines of Go.

The fanout factor 'k' is defaulted to 13, optimised for large networks > 10000 nodes. You should try and reduce this to a more reasonable value if your network is smaller.

## Usage

```go
func main() {
    // some other gossip nodes on the network
    nodes := make([]*net.UDPAddr, 100)

    for i := range nodes {
        // parse the node address and add it to the list
        nodes[i], _ = net.ResolveUDPAddr("udp", fmt.Sprintf("10.64.0.%d:10000", 101+i))
    }

    cfg := &gossip.Config{
        Fanout: gossip.DefaultFanout,         // number of nodes to forward gossip messages to (default: 13)
        Nodes: gossip.DefaultNodeList(nodes), // specifies a node list that random nodes can be selected from
        ListenAddress: "10.64.0.100:10000",   // udp address to bind to
        OnGossip: func(message []byte) {
            // callback to handle gossip messages
            // ensure that the message value is not used outside of this callback!
            m := make([]byte, len(message))
            copy(m, message)

            // do something with m...
        },
    }

    network, err := gossip.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("gossip node started!")

    // send a message to the network
    err = network.Gossip([]byte("hello!"))
    if err != nil {
        log.Fatal(err)
    }
}
```
