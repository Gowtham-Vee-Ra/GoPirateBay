# GoPirateBay - A Barebones Torrent Client in Go

Welcome to **GoPirateBay**, where I attempt to build a **no-frills, no-nonsense** BitTorrent client in **Golang**. Inspired by [this blog post](https://blog.jse.li/posts/torrent/), this project started as an experiment to see **how torrents work under the hood**. Now, I'm here, making mistakes and fixing them as I go!

## What I Have Working (So Far)

- ✅ I can **parse** `.torrent` files (which, as it turns out, is not just some magic file that does stuff).
- ✅ I can **compute SHA-1 hashes** for the torrent info dictionary.
- ✅ I can **contact the tracker** and get a list of peers (who may or may not ghost me).
- ✅ I **display file info** (including size, in human-friendly format because I'm not a robot).
- ⏳ **Next up:** Actually downloading stuff.

## 🏴‍☠️ What Went Horribly Wrong? (Ubuntu & Linux ISO Edition)

### **Trackers**

- Some trackers outright **ignored me**, giving back responses that my decoder couldn't handle.
- My error logs started looking like this:  
  ```
  Error fetching peers: invalid character 'd' looking for beginning of value
  ```
  This means the tracker was saying, "Yeah, no."

### **Unauthorized Requests**

- Some torrents just flat-out **refused to work**, returning errors like:
  ```
  Error fetching peers: requested download is not authorized for use with this tracker.
  ```

### **UDP Trackers (I Don't Support Those (Yet))**

- Many Linux and Ubuntu torrents rely on **UDP trackers**, but my client only speaks **HTTP/HTTPS**.
- This led to cryptic errors like:
  ```
  unsupported protocol scheme "udp"
  ```

### **File Size Detection Bug**

- At first, torrents were reporting **0 bytes** for file size.
- Turns out, I wasn’t parsing the metadata correctly. After some debugging, I **finally** got the correct size.

## Next Steps

To actually **download files**, I need to:

 Improve **Bencode parsing** to handle weird tracker responses.  
 Add **UDP tracker support** so I can actually get peers for more torrents.  
 Implement **better error handling**, so I know *why* something fails (instead of just crying about it).  
 Improve debug logs, because guessing what's wrong is not a debugging strategy.

## 🛠 How to Run (Without Breaking Your Computer)

### Prerequisites

Make sure you have **Go installed**:
```sh
  go version
```
If you don’t have it, grab it from 👉 [https://go.dev/dl/](https://go.dev/dl/)

### Steps to Run

####  Windows
```sh
git clone https://github.com/YOUR_GITHUB_USERNAME/GoPirateBay.git
cd GoPirateBay
go mod tidy
go build -o gotorrent.exe
gotorrent.exe <your-torrent-file>
```

####  Linux & macOS
```sh
git clone https://github.com/YOUR_GITHUB_USERNAME/GoPirateBay.git
cd GoPirateBay
go mod tidy
go build -o gotorrent
./gotorrent <your-torrent-file>
```

##  Sample Output

```
Tracker URL: http://bt1.archive.org:6969/announce
Info Hash: b04de7561b467db42044dc06f70ba8022dbbc58b
File Name: BigBuckBunny_124
File Size: 441.40 MB
Peers: [108.45.89.133:6881]
```

##  What’s Next? 

**1. Actually Downloading the File**  
- Time to stop procrastinating and implement peer-to-peer communication.

**2. Writing Pieces to Disk** 
- Because a bunch of random file pieces in RAM doesn’t make a movie.

**3. Support UDP Trackers**  
- So I can finally use Linux torrents without drama.

**4. CLI Progress Bar** 
- Because watching raw text output scroll endlessly is *so* last century.

**5. Piece Verification**
- Make sure I'm not downloading garbage data.

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

