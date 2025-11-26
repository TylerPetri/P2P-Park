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
