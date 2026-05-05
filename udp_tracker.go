package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"time"
)

const (
	udpConnectMagic   = uint64(0x41727101980)
	udpActionConnect  = uint32(0)
	udpActionAnnounce = uint32(1)
	udpActionError    = uint32(3)
)

// UDPTrackerGetPeers implements the UDP tracker protocol (BEP 15).
// Returns peers, re-announce interval, and any error.
func UDPTrackerGetPeers(trackerURL string, infoHash, peerID [20]byte, fileSize, port int) ([]Peer, int, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid UDP tracker URL: %v", err)
	}

	conn, err := net.DialTimeout("udp", u.Host, 5*time.Second)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to dial UDP tracker %s: %v", u.Host, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))

	connectionID, err := udpConnect(conn)
	if err != nil {
		return nil, 0, fmt.Errorf("UDP connect handshake failed: %v", err)
	}

	return udpAnnounce(conn, connectionID, infoHash, peerID, fileSize, port)
}

func udpConnect(conn net.Conn) (uint64, error) {
	txID := rand.Uint32()

	req := make([]byte, 16)
	binary.BigEndian.PutUint64(req[0:8], udpConnectMagic)
	binary.BigEndian.PutUint32(req[8:12], udpActionConnect)
	binary.BigEndian.PutUint32(req[12:16], txID)

	if _, err := conn.Write(req); err != nil {
		return 0, fmt.Errorf("failed to send connect request: %v", err)
	}

	resp := make([]byte, 16)
	if _, err := conn.Read(resp); err != nil {
		return 0, fmt.Errorf("failed to read connect response: %v", err)
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	if action == udpActionError {
		return 0, fmt.Errorf("tracker error: %s", safeString(resp[8:]))
	}
	if action != udpActionConnect {
		return 0, fmt.Errorf("unexpected action %d in connect response", action)
	}
	if binary.BigEndian.Uint32(resp[4:8]) != txID {
		return 0, fmt.Errorf("transaction ID mismatch in connect response")
	}
	return binary.BigEndian.Uint64(resp[8:16]), nil
}

// udpAnnounce sends the announce request and returns peers plus the interval.
func udpAnnounce(conn net.Conn, connectionID uint64, infoHash, peerID [20]byte, fileSize, port int) ([]Peer, int, error) {
	txID := rand.Uint32()

	req := make([]byte, 98)
	binary.BigEndian.PutUint64(req[0:8], connectionID)
	binary.BigEndian.PutUint32(req[8:12], udpActionAnnounce)
	binary.BigEndian.PutUint32(req[12:16], txID)
	copy(req[16:36], infoHash[:])
	copy(req[36:56], peerID[:])
	binary.BigEndian.PutUint64(req[56:64], 0)
	binary.BigEndian.PutUint64(req[64:72], uint64(fileSize))
	binary.BigEndian.PutUint64(req[72:80], 0)
	binary.BigEndian.PutUint32(req[80:84], 0)
	binary.BigEndian.PutUint32(req[84:88], 0)
	binary.BigEndian.PutUint32(req[88:92], rand.Uint32())
	binary.BigEndian.PutUint32(req[92:96], 50)
	binary.BigEndian.PutUint16(req[96:98], uint16(port))

	if _, err := conn.Write(req); err != nil {
		return nil, 0, fmt.Errorf("failed to send announce request: %v", err)
	}

	resp := make([]byte, 20+6*200)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read announce response: %v", err)
	}
	if n < 20 {
		return nil, 0, fmt.Errorf("announce response too short: %d bytes", n)
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	if action == udpActionError {
		return nil, 0, fmt.Errorf("tracker error: %s", safeString(resp[8:n]))
	}
	if action != udpActionAnnounce {
		return nil, 0, fmt.Errorf("unexpected action %d in announce response", action)
	}
	if binary.BigEndian.Uint32(resp[4:8]) != txID {
		return nil, 0, fmt.Errorf("transaction ID mismatch in announce response")
	}

	interval := int(binary.BigEndian.Uint32(resp[8:12]))
	peers, err := ParsePeers(resp[20:n])
	return peers, interval, err
}

func safeString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
