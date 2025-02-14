package main

import (
	"fmt"
	"io"
	"net"
)

// Handshake represents a BitTorrent handshake message.
type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

// NewHandshake creates a new handshake message.
func NewHandshake(infoHash, peerID [20]byte) *Handshake {
	return &Handshake{
		Pstr:     "BitTorrent protocol",
		InfoHash: infoHash,
		PeerID:   peerID,
	}
}

// Serialize converts a Handshake struct into a byte slice.
func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr)+49)
	buf[0] = byte(len(h.Pstr))
	copy(buf[1:], h.Pstr)
	copy(buf[28:], h.InfoHash[:])
	copy(buf[48:], h.PeerID[:])
	return buf
}

// ReadHandshake reads and parses a handshake response from a peer.
func ReadHandshake(r io.Reader) (*Handshake, error) {
	lengthBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		return nil, fmt.Errorf("failed to read handshake length: %v", err)
	}

	pstrlen := int(lengthBuf[0])
	if pstrlen == 0 || pstrlen > 100 {
		return nil, fmt.Errorf("invalid pstrlen: %d", pstrlen)
	}

	restBuf := make([]byte, 48+pstrlen)
	if _, err := io.ReadFull(r, restBuf); err != nil {
		return nil, fmt.Errorf("failed to read handshake data: %v", err)
	}

	var infoHash, peerID [20]byte
	copy(infoHash[:], restBuf[pstrlen+8:pstrlen+28])
	copy(peerID[:], restBuf[pstrlen+28:])

	return &Handshake{
		Pstr:     string(restBuf[:pstrlen]),
		InfoHash: infoHash,
		PeerID:   peerID,
	}, nil
}

// PerformHandshake performs a handshake with a given peer.
func PerformHandshake(conn net.Conn, infoHash, peerID [20]byte) (*Handshake, error) {
	defer conn.Close()

	handshake := NewHandshake(infoHash, peerID)
	_, err := conn.Write(handshake.Serialize())
	if err != nil {
		return nil, fmt.Errorf("failed to send handshake: %v", err)
	}

	response, err := ReadHandshake(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to receive handshake: %v", err)
	}

	if response.InfoHash != infoHash {
		return nil, fmt.Errorf("handshake info hash mismatch")
	}

	return response, nil
}
