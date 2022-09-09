package gossip

// Config defines configuration for the gossip network
type Config struct {
	// ListenAddress UDP Address to listen on
	ListenAddress string
	// Fanout defines how many nodes a message should be rebroadcasted to
	Fanout int
	// Nodes defines the neighbourhood that random nodes can be selected from for rebroadcast
	Nodes NodeList
	// OnGossip callback for when new gossip is received
	OnGossip func(message []byte)
}
