package swp

// Network describes our network abstraction, and is implemented
// by SimNet and NatsNet.
type Network interface {

	// Send transmits the packet. It is send and pray; no
	// guarantee of delivery is made by the Network.
	Send(pack *Packet, why string) error

	// Listen starts receiving packets addressed to inbox on the returned channel.
	Listen(inbox string) (chan *Packet, error)
}