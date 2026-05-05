package main

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"net/url"

	"github.com/jackpal/bencode-go"
)

// TrackerResponse represents the response from the tracker.
type TrackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

// GeneratePeerID creates a unique 20-byte peer ID.
func GeneratePeerID() [20]byte {
	var peerID [20]byte
	copy(peerID[:], "-GT0001-"+randomID(12))
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

// tryTracker contacts a single tracker and returns peers plus the re-announce interval.
// UDP trackers are dispatched to UDPTrackerGetPeers; others use HTTP.
func tryTracker(trackerURL string, infoHash, peerID [20]byte, fileSize, port int) ([]Peer, int, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid tracker URL: %v", err)
	}
	if u.Scheme == "udp" {
		return UDPTrackerGetPeers(trackerURL, infoHash, peerID, fileSize, port)
	}

	params := url.Values{
		"info_hash":  {string(infoHash[:])},
		"peer_id":    {string(peerID[:])},
		"port":       {fmt.Sprintf("%d", port)},
		"uploaded":   {"0"},
		"downloaded": {"0"},
		"left":       {fmt.Sprintf("%d", fileSize)},
		"compact":    {"1"},
		"numwant":    {"50"},
	}
	u.RawQuery = params.Encode()
	fmt.Println("Tracker Request URL:", u.String())

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, 0, fmt.Errorf("tracker request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("tracker returned HTTP status %d", resp.StatusCode)
	}

	var tr TrackerResponse
	if err := bencode.Unmarshal(resp.Body, &tr); err != nil {
		return nil, 0, fmt.Errorf("failed to parse tracker response: %v", err)
	}

	peers, err := ParsePeers([]byte(tr.Peers))
	return peers, tr.Interval, err
}

// GetPeers tries the primary announce URL then each tier of announce-list,
// returning as soon as any tracker yields at least one peer.
// Also returns the tracker's requested re-announce interval (seconds).
func GetPeers(torrent *Torrent, infoHash [20]byte, peerID [20]byte, port int) ([]Peer, int, error) {
	tried := map[string]bool{}

	try := func(rawURL string) ([]Peer, int, bool) {
		if tried[rawURL] {
			return nil, 0, false
		}
		tried[rawURL] = true
		peers, interval, err := tryTracker(rawURL, infoHash, peerID, torrent.Info.Length, port)
		if err != nil {
			fmt.Printf("⚠ Tracker %s: %v\n", rawURL, err)
			return nil, 0, false
		}
		return peers, interval, len(peers) > 0
	}

	if peers, interval, ok := try(torrent.Announce); ok {
		return peers, interval, nil
	}
	for _, tier := range torrent.AnnounceList {
		for _, u := range tier {
			if peers, interval, ok := try(u); ok {
				return peers, interval, nil
			}
		}
	}
	return nil, 0, fmt.Errorf("no peers found from any tracker")
}
