package proto

// NoiseIdentityPayload is sent inside the Noise handshake payload.
// It binds a user-facing identity to the Noise static key.
type NoiseIdentityPayload struct {
	Name    string `json:"name"`
	UserPub []byte `json:"user_pub"`
}
