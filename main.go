package main

import (
	"fmt"
	"log"
)

func main() {
	torrentFile := "BigBuckBunny_124_archive.torrent"

	// Parse the .torrent file
	t, err := ReadTorrent(torrentFile)
	if err != nil {
		log.Fatalf("Error reading torrent file: %v", err)
	}

	// Print torrent details
	fmt.Println("Tracker URL:", t.Announce)

	infoHash, err := t.ComputeInfoHash()
	if err != nil {
		log.Fatalf("Error computing info hash: %v", err)
	}

	fmt.Printf("Info Hash: %x\n", infoHash)
	fmt.Println("File Name:", t.Info.Name)
	fmt.Println("File Size:", FormatFileSize(t.Info.Length)) // Uses FormatFileSize from torrent.go

	// Fetch Peers from Tracker
	err = GetPeers(t, string(infoHash[:]))
	if err != nil {
		log.Fatalf("Error fetching peers: %v", err)
	}
}
