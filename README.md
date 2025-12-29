# P2P-Park

**A secure, serverless peer-to-peer system in Go.**

P2P-Park is a fully decentralized P2P network where nodes automatically discover each other, authenticate with public-key cryptography, establish encrypted channels, and exchange signed data — **no central server, no trusted coordinator**.

---

## Highlights

### 🔐 Real cryptography
X3DH-style handshake · Noise encryption · XChaCha20-Poly1305 · signed identities

### 🌐 True P2P
Automatic discovery · direct peer connections · no client/server split

### 🧠 Distributed state
Working DHT with signed propagation

### ⚙️ Production Go
Race-safe concurrency · clean peer lifecycle · extensible architecture

---

## Run

```bash
make run
make run NAME=Vee
make run NAME=Sydney
```

## Design Philosophy

This project favors **correctness, clarity, and cryptographic soundness**
over shortcuts or hidden coordination.
Every component is designed to be readable, auditable, and safe to extend
under real network conditions.
