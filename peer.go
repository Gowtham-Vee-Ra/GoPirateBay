package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Peer represents a BitTorrent peer.
type Peer struct {
	IP   string
	Port uint16
}

// Message types for the BitTorrent protocol.
const (
	MsgChoke         = 0
	MsgUnchoke       = 1
	MsgInterested    = 2
	MsgNotInterested = 3
	MsgHave          = 4
	MsgBitfield      = 5
	MsgRequest       = 6
	MsgPiece         = 7
	MsgCancel        = 8
	blockSize        = 16384
)

// ErrKeepAlive is returned by ReadMessage when a keep-alive is received.
var ErrKeepAlive = errors.New("keep-alive")

// ErrChoked is returned by DownloadPiece when the peer chokes us mid-transfer.
var ErrChoked = errors.New("peer choked us")

// hasPiece reports whether bitfield indicates the given piece index is present.
func hasPiece(bitfield []byte, index int) bool {
	byteIndex := index / 8
	if byteIndex >= len(bitfield) {
		return false
	}
	return bitfield[byteIndex]&(1<<(7-uint(index%8))) != 0
}

// setBit sets the bit for index in bitfield.
func setBit(bitfield []byte, index int) {
	byteIndex := index / 8
	if byteIndex < len(bitfield) {
		bitfield[byteIndex] |= 1 << (7 - uint(index%8))
	}
}

// isConnectionError reports whether err means the TCP connection is dead.
func isConnectionError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// ParsePeers decodes the compact binary peer list (6 bytes per peer).
func ParsePeers(data []byte) ([]Peer, error) {
	if len(data)%6 != 0 {
		return nil, errors.New("malformed peers data: length not a multiple of 6")
	}
	peers := make([]Peer, len(data)/6)
	for i := range peers {
		offset := i * 6
		peers[i].IP = net.IP(data[offset : offset+4]).String()
		peers[i].Port = binary.BigEndian.Uint16(data[offset+4 : offset+6])
	}
	return peers, nil
}

// ReadMessage reads a length-prefixed message from the peer.
func ReadMessage(conn net.Conn) (byte, []byte, error) {
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBuf); err != nil {
		return 0, nil, fmt.Errorf("failed to read message length: %w", err)
	}
	length := binary.BigEndian.Uint32(lengthBuf)
	if length == 0 {
		return 0, nil, ErrKeepAlive
	}
	message := make([]byte, length)
	if _, err := io.ReadFull(conn, message); err != nil {
		return 0, nil, fmt.Errorf("failed to read message: %w", err)
	}
	return message[0], message[1:], nil
}

// --- Outgoing message helpers ---

func SendInterested(conn net.Conn) error {
	_, err := conn.Write([]byte{0, 0, 0, 1, MsgInterested})
	return err
}

func SendUnchoke(conn net.Conn) error {
	_, err := conn.Write([]byte{0, 0, 0, 1, MsgUnchoke})
	return err
}

func SendHave(conn net.Conn, pieceIndex int) error {
	msg := make([]byte, 9)
	binary.BigEndian.PutUint32(msg[0:4], 5)
	msg[4] = MsgHave
	binary.BigEndian.PutUint32(msg[5:9], uint32(pieceIndex))
	_, err := conn.Write(msg)
	return err
}

func SendBitfield(conn net.Conn, bitfield []byte) error {
	if len(bitfield) == 0 {
		return nil
	}
	msg := make([]byte, 5+len(bitfield))
	binary.BigEndian.PutUint32(msg[0:4], uint32(1+len(bitfield)))
	msg[4] = MsgBitfield
	copy(msg[5:], bitfield)
	_, err := conn.Write(msg)
	return err
}

func RequestBlock(conn net.Conn, pieceIndex, begin, length int) error {
	msg := make([]byte, 17)
	binary.BigEndian.PutUint32(msg[0:4], 13)
	msg[4] = MsgRequest
	binary.BigEndian.PutUint32(msg[5:9], uint32(pieceIndex))
	binary.BigEndian.PutUint32(msg[9:13], uint32(begin))
	binary.BigEndian.PutUint32(msg[13:17], uint32(length))
	_, err := conn.Write(msg)
	return err
}

// ServeBlock reads a block from disk and sends it as a MsgPiece response.
func ServeBlock(conn net.Conn, fw *FileWriter, pieceIndex, begin, length, pieceLength int) {
	data := make([]byte, length)
	offset := int64(pieceIndex)*int64(pieceLength) + int64(begin)
	if err := fw.ReadAt(data, offset); err != nil {
		return
	}
	msg := make([]byte, 13+length)
	binary.BigEndian.PutUint32(msg[0:4], uint32(9+length))
	msg[4] = MsgPiece
	binary.BigEndian.PutUint32(msg[5:9], uint32(pieceIndex))
	binary.BigEndian.PutUint32(msg[9:13], uint32(begin))
	copy(msg[13:], data)
	conn.Write(msg)
}

// DownloadPiece requests all blocks of a piece, assembles them, validates SHA-1,
// and writes to disk. While waiting for blocks it also handles incoming MsgRequest
// by calling serve (upload), and MsgHave to update peerBitfield.
func DownloadPiece(conn net.Conn, fw *FileWriter, index, pieceLength, totalLength int, expectedHash [20]byte, serve func(idx, begin, length int)) error {
	actualLength := pieceLength
	if (index+1)*pieceLength > totalLength {
		actualLength = totalLength - index*pieceLength
	}
	pieceData := make([]byte, actualLength)

	for begin := 0; begin < actualLength; begin += blockSize {
		length := blockSize
		if begin+length > actualLength {
			length = actualLength - begin
		}
		if err := RequestBlock(conn, index, begin, length); err != nil {
			return fmt.Errorf("failed to request block at offset %d: %v", begin, err)
		}
	}

	received := 0
	for received < actualLength {
		msgType, payload, err := ReadMessage(conn)
		if err != nil {
			if errors.Is(err, ErrKeepAlive) {
				continue
			}
			return err
		}
		switch msgType {
		case MsgPiece:
			if len(payload) < 8 {
				return errors.New("malformed piece message")
			}
			begin := int(binary.BigEndian.Uint32(payload[4:8]))
			data := payload[8:]
			copy(pieceData[begin:], data)
			received += len(data)
		case MsgChoke:
			return ErrChoked
		case MsgRequest:
			if serve != nil && len(payload) >= 12 {
				idx := int(binary.BigEndian.Uint32(payload[0:4]))
				begin := int(binary.BigEndian.Uint32(payload[4:8]))
				length := int(binary.BigEndian.Uint32(payload[8:12]))
				go serve(idx, begin, length)
			}
		}
	}

	if err := ValidatePiece(pieceData, expectedHash); err != nil {
		return fmt.Errorf("piece %d: %v", index, err)
	}

	offset := int64(index) * int64(pieceLength)
	if err := fw.WriteAt(pieceData, offset); err != nil {
		return fmt.Errorf("failed to write piece %d: %v", index, err)
	}
	return nil
}

// waitForUnchoke blocks until the peer sends MsgUnchoke or the connection dies.
func waitForUnchoke(conn net.Conn) error {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		msgType, _, err := ReadMessage(conn)
		if err != nil {
			if errors.Is(err, ErrKeepAlive) {
				continue
			}
			return err
		}
		if msgType == MsgUnchoke {
			return nil
		}
	}
}

// runPeerSession is the shared post-handshake loop used by both outbound
// (ConnectToPeer) and inbound (handleIncoming) connections.
func runPeerSession(conn net.Conn, fw *FileWriter, work *WorkQueue, pieceLength, totalLength int, pieceHashes [][20]byte) error {
	// Announce what we already have.
	if bf := work.GetBitfield(); len(bf) > 0 {
		SendBitfield(conn, bf)
	}

	if err := SendInterested(conn); err != nil {
		return fmt.Errorf("failed to send interested: %v", err)
	}

	// 30-second window for the peer to unchoke us.
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	var peerBitfield []byte
	unchoked := false
	for !unchoked {
		msgType, payload, err := ReadMessage(conn)
		if err != nil {
			if errors.Is(err, ErrKeepAlive) {
				continue
			}
			return fmt.Errorf("error waiting for unchoke: %v", err)
		}
		switch msgType {
		case MsgBitfield:
			peerBitfield = make([]byte, len(payload))
			copy(peerBitfield, payload)
		case MsgHave:
			if len(payload) >= 4 {
				idx := int(binary.BigEndian.Uint32(payload[0:4]))
				if len(peerBitfield) == 0 {
					peerBitfield = make([]byte, (len(pieceHashes)+7)/8)
				}
				setBit(peerBitfield, idx)
			}
		case MsgUnchoke:
			unchoked = true
		}
	}
	conn.SetReadDeadline(time.Time{})

	serve := func(idx, begin, length int) {
		if work.HasPiece(idx) {
			ServeBlock(conn, fw, idx, begin, length, pieceLength)
		}
	}

	for {
		index, ok := work.Get()
		if !ok {
			break
		}
		if len(peerBitfield) > 0 && !hasPiece(peerBitfield, index) {
			work.Requeue(index)
			if work.Remaining() <= endgameThreshold {
				return nil
			}
			continue
		}
		if err := DownloadPiece(conn, fw, index, pieceLength, totalLength, pieceHashes[index], serve); err != nil {
			work.Requeue(index)
			if isConnectionError(err) {
				return fmt.Errorf("connection lost at piece %d: %v", index, err)
			}
			if errors.Is(err, ErrChoked) {
				if waitErr := waitForUnchoke(conn); waitErr != nil {
					return waitErr
				}
				continue
			}
			fmt.Printf("⚠ Piece %d failed: %v\n", index, err)
			continue
		}
		work.Done(index)
		SendHave(conn, index)
		fmt.Printf("📦 Piece %d/%d\n", index+1, work.Total())
	}
	return nil
}

// seedSession handles a connection when we are a complete seeder:
// announces our full bitfield, unchokes interested peers, and serves requests.
func seedSession(conn net.Conn, fw *FileWriter, work *WorkQueue, pieceLength int) {
	SendBitfield(conn, work.GetBitfield())
	for {
		msgType, payload, err := ReadMessage(conn)
		if err != nil {
			if errors.Is(err, ErrKeepAlive) {
				continue
			}
			return
		}
		switch msgType {
		case MsgInterested:
			SendUnchoke(conn)
		case MsgRequest:
			if len(payload) >= 12 {
				idx := int(binary.BigEndian.Uint32(payload[0:4]))
				begin := int(binary.BigEndian.Uint32(payload[4:8]))
				length := int(binary.BigEndian.Uint32(payload[8:12]))
				go ServeBlock(conn, fw, idx, begin, length, pieceLength)
			}
		case MsgChoke:
			return
		}
	}
}

// ConnectToPeer dials a peer, performs the client-side handshake, and runs the session.
func ConnectToPeer(peer Peer, infoHash, peerID [20]byte, fw *FileWriter, work *WorkQueue, pieceLength, totalLength int, pieceHashes [][20]byte) error {
	address := net.JoinHostPort(peer.IP, fmt.Sprintf("%d", peer.Port))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", address, err)
	}
	defer conn.Close()

	handshake, err := PerformHandshake(conn, infoHash, peerID)
	if err != nil {
		return fmt.Errorf("handshake failed with %s: %v", address, err)
	}
	fmt.Printf("✅ Connected to %s | Peer ID: %x\n", address, handshake.PeerID)

	return runPeerSession(conn, fw, work, pieceLength, totalLength, pieceHashes)
}

// handleIncoming processes an inbound peer connection.
func handleIncoming(conn net.Conn, infoHash, peerID [20]byte, fw *FileWriter, work *WorkQueue, pieceLength, totalLength int, pieceHashes [][20]byte) {
	defer conn.Close()
	received, err := PerformServerHandshake(conn, infoHash, peerID)
	if err != nil {
		return
	}
	fmt.Printf("📥 Inbound peer %s | ID: %x\n", conn.RemoteAddr(), received.PeerID)
	if work.IsComplete() {
		seedSession(conn, fw, work, pieceLength)
	} else {
		runPeerSession(conn, fw, work, pieceLength, totalLength, pieceHashes)
	}
}

// startListener accepts inbound peer connections on the given port until ctx is cancelled.
func startListener(ctx context.Context, port int, infoHash, peerID [20]byte, fw *FileWriter, work *WorkQueue, pieceLength, totalLength int, pieceHashes [][20]byte) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Printf("⚠ Could not listen on :%d: %v\n", port, err)
		return
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	fmt.Printf("👂 Listening on :%d\n", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleIncoming(conn, infoHash, peerID, fw, work, pieceLength, totalLength, pieceHashes)
	}
}
