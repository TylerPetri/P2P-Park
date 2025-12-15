# 🌐 P2P-Park (ONGOING)
*A serverless, cryptographically secure peer-to-peer network — written in Go.*

Inspired by the *Nerve (2016)* concept:  
**every device is both a client and a server**.  
No central servers. No rendezvous point.  
A fully decentralized network where peers discover, authenticate, and communicate directly.

P2P-Park is an exploration of **production-grade distributed systems**, **applied cryptography**, and **peer-to-peer networking** — designed to grow into a platform capable of **double-ratchet DMs**, **secure group chats**, and eventually **MLS-style large-scale group messaging**.

---

## 🚀 Status
Actively in development.  
Implementing a Kademlia-like DHT

---

## ✨ Features

### 🔐 Noise-Secured Transport
A fully encrypted transport layer built on the **Noise Protocol Framework**.

- Handshake pattern: **Noise_XX_25519_ChaChaPoly_BLAKE2s**
- Mutual authentication during handshake
- Identity payloads exchanged during connection establishment
- All traffic encrypted via length-prefixed ChaCha20-Poly1305 frames
- Resistant to MITM for authenticated peers

The transport layer alone is suitable for real-world secure messaging systems.

---

### 🆔 Dual Identity Model
P2P-Park cleanly separates:

#### **Device / Network Identity**
- X25519 **Noise static keypair**
- Defines the *network address* of a peer
- Used for routing, peer uniqueness, and connection authentication

#### **User Identity**
- ed25519 **signing keypair**
- Used for:
  - user profiles  
  - signed state (e.g., points module experiments)  
  - future message-level signatures  
- Exchanged via an `/identify` message immediately post-handshake

## 🌐 Vision

For reviewers and hiring managers, the repository highlights:

### ✔ Real cryptographic protocol implementation
- Hand-rolled Noise_XX handshake  
- Identity payload binding  
- Proper use of X25519 + ChaChaPoly + BLAKE2s  

### ✔ Peer-to-peer networking expertise
- Network abstraction  
- Peer state tracking  
- Duplicate-connection resolution  
- Typed gossip message routing  

### ✔ Secure messaging engineering fundamentals
- Layered architecture (transport → identity → app protocol)  
- Extensible message schema  
- Separation of user identity vs device identity  
- Pluggable encrypted channels  

---

PRs and design discussions are welcome.
