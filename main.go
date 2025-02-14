package main

import (
	"fmt"
	"log"
	"os"
)

// main is the entry point of the BitTorrent client.
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gotorrent <torrent-file>")
		return
	}

	torrentFile := os.Args[1]
	torrent, err := ReadTorrent(torrentFile)
	if err != nil {
		log.Fatalf("Failed to parse torrent file: %v", err)
	}

	// Compute the correct InfoHash
	infoHash, err := torrent.ComputeInfoHash()
	if err != nil {
		log.Fatalf("Failed to compute info hash: %v", err)
	}

	fmt.Printf("Tracker URL: %s\n", torrent.Announce)
	fmt.Printf("Info Hash: %x\n", infoHash)
	fmt.Printf("File Name: %s\n", torrent.Info.Name)
	fmt.Printf("File Size: %s\n", FormatFileSize(torrent.Info.Length))

	// Get peer list from tracker
	peers, err := GetPeers(torrent, infoHash)
	if err != nil {
		log.Fatalf("Failed to get peers: %v", err)
	}

	fmt.Printf("📡 Found %d peers\n", len(peers))

	// Generate Peer ID correctly as [20]byte
	peerID := GeneratePeerID()

	// Pass the correctly formatted peerID to ConnectToPeer()
	for _, peer := range peers {
		fmt.Printf("🔗 Connecting to %s:%d...\n", peer.IP, peer.Port)
		err := ConnectToPeer(peer, infoHash, peerID) // No type mismatch now
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		}
	}
}
