package proto

import "encoding/json"

type MessageType string

const (
	MsgHello       MessageType = "hello"
	MsgPeerList    MessageType = "peer_list"
	MsgGossip      MessageType = "gossip" // generics broadcast payload
	MsgIdentify    MessageType = "identify"
	MsgNatRegister MessageType = "nat_register"
	MsgNatRelay    MessageType = "nat_relay"
)

type Envelope struct {
	Type    MessageType     `json:"type"`
	FromID  string          `json:"from_id"`
	Payload json.RawMessage `json:"payload"`
}

// Hello is exchanged on connection setup.
type Hello struct {
	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Protocol string `json:"procol"`
}

// PeerInfo describes another peer we know about.
type PeerInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Addr       string `json:"addr"`
	PublicAddr string `json:"public_addr"`
}

// PeerList is exchanged through gossip to populate other peers' Peerlist.
type PeerList struct {
	Peers []PeerInfo `json:"peers"`
}

// Gossip is our generics "app-level broadcast" payload.
type Gossip struct {
	ID      string          `json:"id"`
	Channel string          `json:"channel"`
	Body    json.RawMessage `json:"body"`
}

// ChatMessage is our payload that is stuffed into Gossip.Body for the global channel
type ChatMessage struct {
	Text      string `json:"text"`
	From      string `json:"from"`
	Timestamp int64  `json:"timestamp"`
}

// Identify is sent by each peer after the transport is secured.
// It tells the remote side "who I am" at the app level.
type Identify struct {
	Name    string `json:"name"`     // display name
	UserPub []byte `json:"user_pub"` // ed25519 public key bytes
}

// PointsSnapshot represents "here is my current score".
// Each identity controls its own score: last higher Version wins.
type PointsSnapshot struct {
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Points   int64  `json:"points"`
	Version  uint64 `json:"version"`
}

// SignedPointsSnapshot wraps a PointsSnapshot with an ed25519 signature.
// PubKey and Signature are []byte; JSON will base64-encode them.
type SignedPointsSnapshot struct {
	Snapshot  PointsSnapshot `json:"snapshot"`
	PubKey    []byte         `json:"pub_key"`
	Signature []byte         `json:"sig"`
}

// EncryptedMessage wraps AEAD nonce + ciphertext for encrypted channels.
type EncryptedMessage struct {
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// NatRegister is sent from a client to a SeedNode to say:
// "I'm online as this user; please be able to route to me."
type NatRegister struct {
	UserID string `json:"user_id"` // hex(ed25519 pub) or your userID
	Name   string `json:"name"`    // display name (optional, for debugging)
}

// NatRelay is sent from a client to a SeedNode, and the SeedNode forwards it
// to the target user if they're registered and connected.
type NatRelay struct {
	ToUserID string          `json:"to_user_id"` // target user
	Payload  json.RawMessage `json:"payload"`    // opaque; app defines
}
