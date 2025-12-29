# Security Policy

P2P-Park is a cryptography-first, fully decentralized peer-to-peer system.
Security is treated as a core feature, not an afterthought.

## Design Principles

- **No trusted servers**
  All nodes authenticate using public-key identity.
  There is no central authority or coordinator.

- **Authenticated key exchange**
  Peers establish shared secrets using an X3DH-style handshake,
  providing identity binding and resistance to MITM attacks.

- **Encrypted transport**
  All peer communication occurs over Noise-encrypted channels using
  modern AEAD primitives (XChaCha20-Poly1305).

- **Signed state**
  Data propagated through the DHT is cryptographically signed and verified
  to prevent spoofing or unauthorized mutation.

## Threat Model (Non-Exhaustive)

This project explicitly considers:
- Passive and active network observers
- Man-in-the-middle attacks
- Replay attacks
- Unauthorized peer impersonation
- Malicious peers attempting to inject invalid state

This project does *not* currently attempt to defend against:
- Global adversaries with full network visibility
- Denial-of-service at Internet scale

## Reporting Issues

If you discover a security issue, please open a private issue or contact
the maintainer directly. Responsible disclosure is appreciated.
