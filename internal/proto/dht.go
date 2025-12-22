package proto

const MsgDHT = "dht"

// DHTWire is the single payload for all DHT traffic.
type DHTWire struct {
	// Kind is one of:
	// "PING", "PONG", "FIND_NODE", "NODES",
	// "STORE", "STORE_RESULT",
	// "FIND_VALUE", "VALUE"
	Kind string `json:"kind"`

	// RPC correlation
	RPCID string `json:"rpc_id,omitempty"`

	// The lookup target for FIND_NODE (32-byte node id, hex string)
	Target string `json:"target,omitempty"`

	// Returned nodes for NODES (and sometimes VALUE fallback)
	Nodes []DHTNode `json:"nodes,omitempty"`

	// Key for STORE/FIND_VALUE/VALUE (32-byte key, hex string)
	Key string `json:"key,omitempty"`

	// Record for STORE/VALUE
	Record *DHTRecord `json:"record,omitempty"`

	// STORE_RESULT
	OK    bool   `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

type DHTNode struct {
	NodeID string `json:"node_id"` // 64 hex chars; sha256(pubkey)
	PeerID string `json:"peer_id"` // 64 hex chars; hex(pubkey)
	Addr   string `json:"addr"`    // host:port
	Name   string `json:"name,omitempty"`
}

// DHTRecord supports immutable + mutable records.
// Immutable: Key = hash(Value), no Sig, PubKey, Seq required.
// Mutable: Key = hash(PubKey||Name), requires PubKey, Seq, Sig.
type DHTRecord struct {
	Type string `json:"type"` // "IMMUTABLE" | "MUTABLE"

	// Value payload
	Value []byte `json:"value,omitempty"`

	// Metadata
	CreatedUnix int64 `json:"created_unix,omitempty"`
	ExpiresUnix int64 `json:"expires_unix,omitempty"`

	// Mutable-only
	PubKey []byte `json:"pubkey,omitempty"`
	Seq    uint64 `json:"seq,omitempty"`
	Sig    []byte `json:"sig,omitempty"`

	// Optional “name/namespace” for mutable keys (helps debugging, optional)
	Name string `json:"name,omitempty"`
}
