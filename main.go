package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	outputDir = flag.String("o", ".", "output directory")
	listenPort = flag.Int("port", 6881, "port to listen on for incoming peers")
	maxPeers  = flag.Int("max-peers", 50, "maximum outbound peer connections")
	noResume  = flag.Bool("no-resume", false, "ignore existing files and re-download everything")
	seed      = flag.Bool("seed", false, "keep seeding after download completes")
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <torrent-file>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	torrent, err := ReadTorrent(flag.Arg(0))
	if err != nil {
		log.Fatalf("Failed to read torrent file: %v", err)
	}

	infoHash, err := torrent.ComputeInfoHash()
	if err != nil {
		log.Fatalf("Failed to compute info hash: %v", err)
	}

	fmt.Printf("Tracker URL:  %s\n", torrent.Announce)
	fmt.Printf("Info Hash:    %x\n", infoHash)
	fmt.Printf("File Name:    %s\n", torrent.Info.Name)
	fmt.Printf("File Size:    %s\n", FormatFileSize(torrent.Info.Length))

	peerID := GeneratePeerID()
	peers, interval, err := GetPeers(torrent, infoHash, peerID, *listenPort)
	if err != nil {
		log.Fatalf("Failed to get peers: %v", err)
	}
	fmt.Printf("📡 Found %d peers (re-announce in %ds)\n", len(peers), interval)

	fw, err := NewFileWriter(torrent, *outputDir)
	if err != nil {
		log.Fatalf("Failed to open output files: %v", err)
	}
	defer fw.Close()

	pieceHashes := torrent.GetPieceHashes()
	fmt.Printf("🧩 %d pieces to download\n", len(pieceHashes))

	// Resume: verify pieces already on disk before connecting to any peers.
	var preCompleted []int
	if !*noResume {
		fmt.Print("🔍 Checking for existing pieces...")
		preCompleted = checkResume(fw, pieceHashes, torrent.Info.PieceLength, torrent.Info.Length)
		fmt.Printf(" %d/%d already done\n", len(preCompleted), len(pieceHashes))
	}

	work := NewWorkQueue(len(pieceHashes), preCompleted)
	if work.IsComplete() {
		fmt.Println("✅ Already complete.")
		maybeSeed(fw, work, torrent.Info.PieceLength, infoHash, peerID, pieceHashes, torrent.Info.Length)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startListener(ctx, *listenPort, infoHash, peerID, fw, work, torrent.Info.PieceLength, torrent.Info.Length, pieceHashes)

	// Progress reporter: prints speed and ETA every 5 seconds.
	go progressLoop(ctx, work, torrent.Info.PieceLength)

	// Track which peer addresses we have seen to avoid duplicate connections on re-announce.
	var seenPeers sync.Map
	var wg sync.WaitGroup

	connectNew := func(batch []Peer) {
		cap := *maxPeers
		for _, p := range batch {
			addr := fmt.Sprintf("%s:%d", p.IP, p.Port)
			if _, loaded := seenPeers.LoadOrStore(addr, struct{}{}); loaded {
				continue
			}
			if cap <= 0 {
				break
			}
			cap--
			wg.Add(1)
			go func(peer Peer) {
				defer wg.Done()
				fmt.Printf("🔗 Connecting to %s:%d...\n", peer.IP, peer.Port)
				if err := ConnectToPeer(peer, infoHash, peerID, fw, work, torrent.Info.PieceLength, torrent.Info.Length, pieceHashes); err != nil {
					fmt.Printf("❌ %s:%d: %v\n", peer.IP, peer.Port, err)
				}
			}(p)
		}
	}

	connectNew(peers)

	// Re-announce goroutine: refresh the peer list on the tracker's schedule.
	if interval <= 0 {
		interval = 1800
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(interval) * time.Second):
			}
			if work.IsComplete() {
				return
			}
			newPeers, newInterval, err := GetPeers(torrent, infoHash, peerID, *listenPort)
			if err != nil {
				continue
			}
			if newInterval > 0 {
				interval = newInterval
			}
			fmt.Printf("🔄 Re-announced: %d peers\n", len(newPeers))
			connectNew(newPeers)
		}
	}()

	wg.Wait()
	cancel()
	fmt.Println("✅ Download complete")

	maybeSeed(fw, work, torrent.Info.PieceLength, infoHash, peerID, pieceHashes, torrent.Info.Length)
}

// checkResume reads every piece from disk in parallel and returns indices that
// pass SHA-1 verification.
func checkResume(fw *FileWriter, hashes [][20]byte, pieceLength, totalLength int) []int {
	var mu sync.Mutex
	var done []int
	var wg sync.WaitGroup

	for i, h := range hashes {
		wg.Add(1)
		go func(idx int, expected [20]byte) {
			defer wg.Done()
			actual := pieceLength
			if (idx+1)*pieceLength > totalLength {
				actual = totalLength - idx*pieceLength
			}
			buf := make([]byte, actual)
			if err := fw.ReadAt(buf, int64(idx)*int64(pieceLength)); err != nil {
				return
			}
			if ValidatePiece(buf, expected) == nil {
				mu.Lock()
				done = append(done, idx)
				mu.Unlock()
			}
		}(i, h)
	}
	wg.Wait()
	return done
}

// progressLoop prints download speed and ETA every 5 seconds.
func progressLoop(ctx context.Context, work *WorkQueue, pieceLength int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	lastDone := work.Completed()
	lastTime := time.Now()
	var speed float64 // bytes/sec, EMA

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			done := work.Completed()
			total := work.Total()
			now := time.Now()
			elapsed := now.Sub(lastTime).Seconds()

			if elapsed > 0 {
				instant := float64(done-lastDone) * float64(pieceLength) / elapsed
				if speed == 0 {
					speed = instant
				} else {
					speed = 0.3*instant + 0.7*speed
				}
			}
			lastDone = done
			lastTime = now

			pct := float64(done) / float64(total) * 100
			eta := "ETA --:--"
			if speed > 0 && done < total {
				remaining := float64(total-done) * float64(pieceLength) / speed
				eta = "ETA " + fmtDuration(remaining)
			}
			fmt.Printf("📊 %d/%d (%.1f%%) | %s | %s\n", done, total, pct, fmtSpeed(speed), eta)
		}
	}
}

// maybeSeed keeps the process alive for seeding if -seed was passed.
func maybeSeed(fw *FileWriter, work *WorkQueue, pieceLength int, infoHash, peerID [20]byte, pieceHashes [][20]byte, totalLength int) {
	if !*seed {
		return
	}
	fmt.Println("🌱 Seeding — press Ctrl+C to stop")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go startListener(ctx, *listenPort, infoHash, peerID, fw, work, pieceLength, totalLength, pieceHashes)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
