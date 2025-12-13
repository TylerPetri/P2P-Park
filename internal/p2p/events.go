package p2p

type EventType string

const (
	EventPeerConnected    EventType = "peer_connected"
	EventPeerDisconnected EventType = "peer_disconnected"
)

type Event struct {
	Type     EventType
	PeerID   string
	PeerAddr string
	PeerName string
	Err      string
}
