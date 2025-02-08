package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
)

func downloadFromPeer(peer string, t *Torrent) error {
	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Println("Connected to peer:", peer)

	// Simplified handshake
	handshake := make([]byte, 68)
	handshake[0] = 19
	copy(handshake[1:], "BitTorrent protocol")
	copy(handshake[28:], t.Info.Name)
	copy(handshake[48:], "00112233445566778899")
	conn.Write(handshake)

	response := make([]byte, 68)
	conn.Read(response)
	fmt.Println("Handshake complete with", peer)

	// Request first piece
	pieceRequest := make([]byte, 17)
	binary.BigEndian.PutUint32(pieceRequest[1:], 13) // Length
	pieceRequest[5] = 6                              // Request type
	binary.BigEndian.PutUint32(pieceRequest[9:], 0)  // Piece index
	binary.BigEndian.PutUint32(pieceRequest[13:], 0)
	binary.BigEndian.PutUint32(pieceRequest[17:], 16384)
	conn.Write(pieceRequest)

	// Read response
	data := make([]byte, 16384)
	io.ReadFull(conn, data)
	fmt.Println("Received first piece")

	// Save file
	file, err := os.Create(t.Info.Name)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Write(data)

	return nil
}
