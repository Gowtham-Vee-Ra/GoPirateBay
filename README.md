# GoPirateBay

A BitTorrent client written in Go from scratch. Inspired by [this blog post](https://blog.jse.li/posts/torrent/), this project started as an exercise to understand how torrents work under the hood — parsing `.torrent` files, talking to trackers, doing the handshake with peers, downloading and verifying pieces. What started as a simple experiment grew into a fairly complete client.

## Features

- Parses `.torrent` files (single-file and multi-file)
- HTTP and UDP tracker support (BEP 3, BEP 15)
- Compact peer list decoding (6 bytes per peer)
- Full piece download with 16 KB block pipelining and SHA-1 verification
- Endgame mode: last 5 pieces are broadcast to all connected peers simultaneously
- Resume: SHA-1-verifies existing pieces on disk before connecting to any peer
- Upload/seeding: serves blocks to peers that request them, full seed mode after download
- Inbound peer connections (listen socket)
- Tracker re-announce on the interval the tracker specifies
- Speed and ETA display (EMA-smoothed, updated every 5 seconds)

## Building

Requires Go 1.21 or later (uses `slices` from the standard library).

```
go mod tidy
go build -o gotorrent
```

## Usage

```
./gotorrent [options] <torrent-file>
```

Options:

| Flag | Default | Description |
|------|---------|-------------|
| `-o` | `.` | Output directory |
| `-port` | `6881` | Port to listen on for inbound peers |
| `-max-peers` | `50` | Maximum outbound peer connections |
| `-no-resume` | false | Ignore existing files and re-download everything |
| `-seed` | false | Keep seeding after download completes |

### Example

Any publicly available `.torrent` file works. A few good ones for testing:

- **Big Buck Bunny** (movie, ~276 MB) — widely seeded, good for testing sustained throughput
  Download the `.torrent` from `https://webtorrent.io/torrents/big-buck-bunny.torrent`

- **Debian netinstall ISO** (~630 MB) — official tracker, very reliable
  `https://cdimage.debian.org/debian-cd/current/amd64/bt-cd/debian-12.9.0-amd64-netinst.iso.torrent`

- **Ubuntu desktop ISO** (~5 GB) — good stress test for multi-peer downloads
  `https://releases.ubuntu.com/22.04/ubuntu-22.04.5-desktop-amd64.iso.torrent`

```
./gotorrent debian-12.9.0-amd64-netinst.iso.torrent
./gotorrent -o ~/Downloads ubuntu-22.04.5-desktop-amd64.iso.torrent
./gotorrent -seed -port 6882 big-buck-bunny.torrent
```

### Sample output

```
Tracker URL:  http://bttracker.debian.org:6969/announce
Info Hash:    dc8a9285b8f13d517f982e8d82b7e0f5b18a8a45
File Name:    debian-12.9.0-amd64-netinst.iso
File Size:    628.00 MB
Found 47 peers (re-announce in 1800s)
635 pieces to download
Checking for existing pieces... 0/635 already done
Listening on :6881
Connecting to 91.121.80.179:51413...
Connected to 91.121.80.179:51413 | Peer ID: 2d5452323934302d...
Piece 1/635
Piece 2/635
...
128/635 (20.2%) | 3.4 MB/s | ETA 2m38s
```

## Architecture

| File | Responsibility |
|------|----------------|
| `main.go` | Entry point, CLI flags, resume check, re-announce goroutine, progress loop |
| `torrent.go` | `.torrent` file parsing, info hash computation, piece hash extraction |
| `tracker.go` | HTTP tracker requests, peer list parsing, multi-tracker fallback |
| `udp_tracker.go` | UDP tracker protocol (BEP 15) |
| `peer.go` | Wire protocol messages, piece download, upload, session management |
| `handshake.go` | BitTorrent handshake (client and server side) |
| `files.go` | FileWriter: maps global byte offsets to one or more output files |
| `queue.go` | Thread-safe work queue with endgame mode and resume support |
| `progress.go` | Speed and duration formatting helpers |

## Protocol notes

- Pieces are downloaded by sending all block requests for a piece upfront (pipelining), then reading responses.
- Re-choke during a piece download is handled by waiting for a new unchoke before retrying.
- In endgame mode the work queue rotates pieces rather than popping them, so multiple peers race to finish the last few pieces. `Done()` is idempotent to handle the resulting duplicate completions safely.
- The listener accepts inbound connections and runs the same session logic as outbound connections.

## License

MIT
