package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// fileSegment maps one file to its range in the global byte space.
type fileSegment struct {
	path        string
	globalStart int64
	globalEnd   int64
	file        *os.File
}

// FileWriter maps global byte offsets to the correct file(s), handling both
// single-file and multi-file torrents transparently.
type FileWriter struct {
	mu       sync.Mutex
	segments []fileSegment
}

// NewFileWriter opens (or creates) all files described by the torrent under
// outputDir. Existing files are not truncated so resume works naturally.
func NewFileWriter(torrent *Torrent, outputDir string) (*FileWriter, error) {
	fw := &FileWriter{}

	if len(torrent.Info.Files) == 0 {
		path := filepath.Join(outputDir, torrent.Info.Name)
		f, err := openOrCreate(path, int64(torrent.Info.Length))
		if err != nil {
			return nil, err
		}
		fw.segments = []fileSegment{{
			path:        path,
			globalStart: 0,
			globalEnd:   int64(torrent.Info.Length),
			file:        f,
		}}
		return fw, nil
	}

	// Multi-file: create directory tree and open every file.
	baseDir := filepath.Join(outputDir, torrent.Info.Name)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}

	var offset int64
	for _, tf := range torrent.Info.Files {
		parts := append([]string{baseDir}, tf.Path...)
		path := filepath.Join(parts...)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			fw.Close()
			return nil, err
		}
		f, err := openOrCreate(path, int64(tf.Length))
		if err != nil {
			fw.Close()
			return nil, err
		}
		fw.segments = append(fw.segments, fileSegment{
			path:        path,
			globalStart: offset,
			globalEnd:   offset + int64(tf.Length),
			file:        f,
		})
		offset += int64(tf.Length)
	}
	return fw, nil
}

// openOrCreate opens a file for read/write, creating it if necessary.
// Only truncates (pre-allocates) when the on-disk size differs from expected.
func openOrCreate(path string, expectedSize int64) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.Size() != expectedSize {
		if err := f.Truncate(expectedSize); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to allocate %s: %v", path, err)
		}
	}
	return f, nil
}

// WriteAt writes data at globalOffset, splitting across file boundaries as needed.
func (fw *FileWriter) WriteAt(data []byte, globalOffset int64) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.transfer(data, globalOffset, false)
}

// ReadAt reads into buf starting at globalOffset, crossing file boundaries.
func (fw *FileWriter) ReadAt(buf []byte, globalOffset int64) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.transfer(buf, globalOffset, true)
}

// transfer is the shared read/write helper.
func (fw *FileWriter) transfer(buf []byte, globalOffset int64, read bool) error {
	remaining := buf
	pos := globalOffset
	for len(remaining) > 0 {
		seg := fw.segmentAt(pos)
		if seg == nil {
			return fmt.Errorf("no file segment for offset %d", pos)
		}
		localOffset := pos - seg.globalStart
		n := seg.globalEnd - pos
		if int64(len(remaining)) < n {
			n = int64(len(remaining))
		}
		var err error
		if read {
			_, err = seg.file.ReadAt(remaining[:n], localOffset)
		} else {
			_, err = seg.file.WriteAt(remaining[:n], localOffset)
		}
		if err != nil {
			return err
		}
		remaining = remaining[n:]
		pos += n
	}
	return nil
}

func (fw *FileWriter) segmentAt(offset int64) *fileSegment {
	for i := range fw.segments {
		s := &fw.segments[i]
		if offset >= s.globalStart && offset < s.globalEnd {
			return s
		}
	}
	return nil
}

func (fw *FileWriter) Close() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	for _, seg := range fw.segments {
		if seg.file != nil {
			seg.file.Close()
		}
	}
}
