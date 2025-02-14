package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

// Peer represents a BitTorrent peer.
type Peer struct {
	IP   net.IP
	Port uint16
}

// ParsePeers extracts peer addresses from a raw tracker response.
func ParsePeers(peersBinary []byte) ([]Peer, error) {
	const peerSize = 6
	if len(peersBinary)%peerSize != 0 {
		return nil, errors.New("malformed peers data")
	}

	numPeers := len(peersBinary) / peerSize
	peers := make([]Peer, numPeers)
	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i] = Peer{
			IP:   net.IP(peersBinary[offset : offset+4]),
			Port: binary.BigEndian.Uint16(peersBinary[offset+4 : offset+6]),
		}
	}

	return peers, nil
}

// ConnectToPeer establishes a TCP connection with a peer and performs the handshake.
func ConnectToPeer(peer Peer, infoHash, peerID [20]byte) error {
	address := fmt.Sprintf("%s:%d", peer.IP.String(), peer.Port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %v", address, err)
	}
	defer conn.Close()

	handshake, err := PerformHandshake(conn, infoHash, peerID)
	if err != nil {
		return fmt.Errorf("handshake failed with peer %s: %v", address, err)
	}

	fmt.Printf("✅ Successfully connected to %s | Peer ID: %x\n", address, handshake.PeerID)
	return nil
}
