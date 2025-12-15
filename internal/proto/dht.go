package proto

// Add this constant alongside your existing Msg* constants.
const MsgDHT = "dht"

// DHTWire is the single payload for all DHT traffic.
// Keep it flat + explicit for forwards-compat.
type DHTWire struct {
	// Kind is one of: "PING", "PONG", "FIND_NODE", "NODES"
	Kind string `json:"kind"`

	// RPC correlation (optional but very useful once you do iterative lookups)
	RPCID string `json:"rpc_id,omitempty"`

	// The lookup target for FIND_NODE (32-byte node id, hex string)
	Target string `json:"target,omitempty"`

	// Returned nodes for NODES
	Nodes []DHTNode `json:"nodes,omitempty"`
}

type DHTNode struct {
	ID   string `json:"id"`   // 64 hex chars (32 bytes)
	Addr string `json:"addr"` // host:port
	Name string `json:"name,omitempty"`
}
