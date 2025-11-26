package proto

import "encoding/json"

type MessageType string

const (
	MsgHello    MessageType = "hello"
	MsgPeerList MessageType = "peer_list"
	MsgGossip   MessageType = "gossip" // generics broadcast payload
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

type PeerInfo struct {
	ID   string `json:"id"`
	Addr string `json:"addr"`
	Name string `json:"name"`
}

// PeerInfo describes another peer we know about.
type PeerList struct {
	Peers []PeerInfo `json:"peers"`
}

// Gossip is our generics "app-level broadcast" payload.
type Gossip struct {
	Channel string          `json:"channel"`
	Body    json.RawMessage `json:"body"`
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

// SignedEnvelope wraps an Envelope with a signature and pubkey.
// This is what travels on the wire.
type SignedEnvelope struct {
	Envelope  Envelope `json:"env"`
	PubKey    []byte   `json:"pub_key"`
	Signature []byte   `json:"sig"`
}
