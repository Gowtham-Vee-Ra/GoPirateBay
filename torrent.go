package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

// Torrent struct represents parsed .torrent metadata
type Torrent struct {
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list"`
	Info         struct {
		Name        string `bencode:"name"`
		PieceLength int    `bencode:"piece length"`
		Pieces      string `bencode:"pieces"`
		Length      int    `bencode:"length,omitempty"` // Single-file torrents
		Files       []struct {
			Length int      `bencode:"length"`
			Path   []string `bencode:"path"`
		} `bencode:"files,omitempty"` // Multi-file torrents
	} `bencode:"info"`
}

// ReadTorrent parses a .torrent file
func ReadTorrent(filePath string) (*Torrent, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var t Torrent
	err = bencode.Unmarshal(file, &t)
	if err != nil {
		return nil, err
	}

	// Handle multi-file torrents by summing up file sizes
	if t.Info.Length == 0 && len(t.Info.Files) > 0 {
		totalSize := 0
		for _, file := range t.Info.Files {
			totalSize += file.Length
		}
		t.Info.Length = totalSize
	}

	return &t, nil
}

// ComputeInfoHash calculates SHA-1 hash of the 'info' dictionary
func (t *Torrent) ComputeInfoHash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, t.Info)
	if err != nil {
		return [20]byte{}, fmt.Errorf("failed to bencode info dictionary: %v", err)
	}

	return sha1.Sum(buf.Bytes()), nil
}

// GetPieceHashes splits the concatenated SHA-1 piece hashes into individual entries.
func (t *Torrent) GetPieceHashes() [][20]byte {
	raw := []byte(t.Info.Pieces)
	hashes := make([][20]byte, len(raw)/20)
	for i := range hashes {
		copy(hashes[i][:], raw[i*20:(i+1)*20])
	}
	return hashes
}

// FormatFileSize converts bytes to a readable format
func FormatFileSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/1024/1024)
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/1024/1024/1024)
}
