# GoPirateBay - A Barebones Torrent Client in Go

Welcome to **GoPirateBay**, where I attempt to build a **no-frills, no-nonsense** BitTorrent client in **Golang**. Inspired by [this blog post](https://blog.jse.li/posts/torrent/), this project started as an experiment to see **how torrents work under the hood**. Now, after countless tracker failures, handshake misfires, and some deep dives into network protocols, I finally have something functional.

## 🚀 What I Have Working (So Far)

- ✅ **Parsing `.torrent` files** - Extracting metadata, info hash, and announce URLs.
- ✅ **Tracker communication** - Requesting and retrieving peer lists successfully.
- ✅ **Peer discovery** - Connecting to peers over TCP and validating handshakes.
- ✅ **Error handling** - Cleaning up weird tracker responses and improving logs.
- ✅ **CLI improvements** - Better formatted output, showing peers and tracker responses.
- ⏳ **Next up:** Actually downloading file pieces (yes, the crucial part).

## 🏴‍☠️ What Went Wrong (And How I Fixed It)

### **1. Trackers Ignoring Requests**
- Some trackers completely ignored my client’s requests, or sent back responses I couldn’t parse.
- **Fix:** Improved how I parse tracker responses, handling unexpected formats.

### **2. Connection Issues with Peers**
- Some peers wouldn't connect, some refused the handshake, and others ghosted me.
- **Fix:** Implemented better TCP connection handling, including retries and logging failed attempts.

### **3. File Size Detection Was Broken**
- At first, torrents were reporting **0 bytes** for file size. Kind of important to fix that.
- **Fix:** Debugged metadata parsing and made sure the correct fields were extracted.

### **4. Missing UDP Tracker Support**
- A lot of popular torrents rely on UDP trackers, but my client only supported HTTP.
- **Fix (Upcoming):** Implement UDP tracker communication for broader compatibility.

## Next Steps

To actually **download files**, I need to:

Implement **piece downloading** - fetching and storing file pieces correctly.
Add **UDP tracker support** - for better peer discovery.
Improve **peer connection handling** - support multiple simultaneous connections.
Implement **piece verification** - ensuring we download valid data.
Introduce a **CLI progress bar** - because watching raw logs is painful.

## 🛠 How to Run

### Prerequisites

Make sure you have **Go installed**:
```sh
go version
```
If you don’t have it, grab it from 👉 [https://go.dev/dl/](https://go.dev/dl/)

### Steps to Run

#### Windows
```sh
git clone https://github.com/veggiedefender/torrent-client.git
cd GoPirateBay
go mod tidy
go build -o gotorrent.exe
gotorrent.exe <your-torrent-file>
```

#### Linux & macOS
```sh
git clone https://github.com/veggiedefender/torrent-client.git
cd GoPirateBay
go mod tidy
go build -o gotorrent
./gotorrent <your-torrent-file>
```

## Sample Output

```
Tracker URL: http://bt1.archive.org:6969/announce
Info Hash: b04de7561b467db42044dc06f70ba8022dbbc58b
File Name: BigBuckBunny_124
File Size: 441.40 MB
📡 Found 50 peers
🔗 Connecting to 67.146.22.212:1024...
✅ Successfully connected to 67.146.22.212:1024 | Peer ID: 2d5554333630572d5cb83c94faf97e0c8312990d
...
```

## What’s Next?

✔ **1. Downloading Pieces**
✔ **2. Writing Data to Disk**  
✔ **3. Supporting UDP Trackers**  
✔ **4. Adding Piece Verification**  
✔ **5. Implementing CLI Progress Bar**  

## 🤝 Contributing

Want to help make this better?

1. Fork the repo.  
2. Create a branch:
   ```sh
   git checkout -b feature-name
   ```
3. Commit changes:
   ```sh
   git commit -m "Added feature XYZ"
   ```
4. Push & create a **Pull Request**.  

## 📜 License

This project is **open-source** under the **MIT License**.

---