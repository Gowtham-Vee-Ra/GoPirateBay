package main

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"net/url"

	"github.com/jackpal/bencode-go"
)

// TrackerResponse represents the response from the tracker
type TrackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

// GeneratePeerID creates a unique 20-byte peer ID for tracker communication
func GeneratePeerID() [20]byte {
	var peerID [20]byte
	copy(peerID[:], "-GT0001-"+randomID(12)) // -GT0001- (Client identifier) + 12 random characters
	return peerID
}

func randomID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic("Failed to generate random peer ID")
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// GetPeers contacts the tracker and fetches a peer list
func GetPeers(torrent *Torrent, infoHash [20]byte) ([]Peer, error) {
	trackerURL, err := url.Parse(torrent.Announce)
	if err != nil {
		return nil, fmt.Errorf("invalid tracker URL: %v", err)
	}

	peerID := GeneratePeerID()

	params := url.Values{
		"info_hash":  {string(infoHash[:])},
		"peer_id":    {string(peerID[:])}, // Convert [20]byte to string
		"port":       {"6881"},
		"uploaded":   {"0"},
		"downloaded": {"0"},
		"left":       {fmt.Sprintf("%d", torrent.Info.Length)},
		"compact":    {"1"},
	}

	trackerURL.RawQuery = params.Encode()
	fmt.Println("Tracker Request URL:", trackerURL.String())

	// Send HTTP GET request to tracker
	resp, err := http.Get(trackerURL.String())
	if err != nil {
		return nil, fmt.Errorf("tracker request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker returned HTTP status %d", resp.StatusCode)
	}

	var trackerResp TrackerResponse
	err = bencode.Unmarshal(resp.Body, &trackerResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracker response: %v", err)
	}

	// Use ParsePeers() from peer.go
	peers, err := ParsePeers([]byte(trackerResp.Peers))
	if err != nil {
		return nil, fmt.Errorf("failed to parse peers: %v", err)
	}

	return peers, nil
}
