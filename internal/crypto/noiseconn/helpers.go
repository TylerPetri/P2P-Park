package noiseconn

import (
	"encoding/binary"
	"fmt"
	"io"
)

// writeHandshakeMsg sends a length-prefixed handshake message.
func writeHandshakeMsg(w io.Writer, msg []byte) error {
	var lenBuf [2]byte
	if len(msg) > 0xffff {
		return fmt.Errorf("handshake message too long")
	}
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(msg)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	return nil
}

// readHandshakeMsg reads a single length-prefixed handshake message.
func readHandshakeMsg(r io.Reader) ([]byte, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint16(lenBuf[:])
	if n == 0 {
		return nil, fmt.Errorf("invalid handshake message length")
	}
	msg := make([]byte, n)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
