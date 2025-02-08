package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// GetPeers fetches peers from the tracker
func GetPeers(t *Torrent, infoHash string) error {
	baseURL, err := url.Parse(t.Announce)
	if err != nil {
		return fmt.Errorf("invalid tracker URL: %v", err)
	}

	// Building the announce request URL
	query := baseURL.Query()
	query.Set("info_hash", infoHash)
	query.Set("peer_id", "-GT0001-001122334455") // Generic peer_id
	query.Set("port", "6881")
	query.Set("uploaded", "0")
	query.Set("downloaded", "0")
	query.Set("left", fmt.Sprintf("%d", t.Info.Length))
	query.Set("compact", "1")

	baseURL.RawQuery = query.Encode()

	fmt.Println("Tracker URL:", baseURL.String())

	// Making the request
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return fmt.Errorf("error contacting tracker: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading tracker response: %v", err)
	}

	fmt.Println("Tracker Response:", string(body))
	return nil
}
