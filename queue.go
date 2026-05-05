package main

import (
	"slices"
	"sync"
)

const endgameThreshold = 5

// WorkQueue is a thread-safe piece queue with bitfield tracking, endgame support,
// and idempotent Done() for race-safe endgame completion.
type WorkQueue struct {
	mu         sync.Mutex
	cond       *sync.Cond
	pending    []int
	total      int
	completed  int
	closed     bool
	downloaded []byte // our bitfield of successfully downloaded pieces
}

// NewWorkQueue creates a queue for numPieces pieces, pre-marking any indices in
// preCompleted as already done (used for resume).
func NewWorkQueue(numPieces int, preCompleted []int) *WorkQueue {
	skip := make(map[int]bool, len(preCompleted))
	for _, i := range preCompleted {
		skip[i] = true
	}

	pending := make([]int, 0, numPieces-len(preCompleted))
	for i := 0; i < numPieces; i++ {
		if !skip[i] {
			pending = append(pending, i)
		}
	}

	downloaded := make([]byte, (numPieces+7)/8)
	for _, i := range preCompleted {
		setBit(downloaded, i)
	}

	wq := &WorkQueue{
		pending:    pending,
		total:      numPieces,
		completed:  len(preCompleted),
		downloaded: downloaded,
	}
	wq.cond = sync.NewCond(&wq.mu)
	if len(pending) == 0 {
		wq.closed = true
	}
	return wq
}

// Get blocks until a piece is available or all pieces are done.
// In endgame mode the piece is rotated rather than popped so multiple peers
// can race to finish the last few pieces.
func (wq *WorkQueue) Get() (int, bool) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	for len(wq.pending) == 0 && !wq.closed {
		wq.cond.Wait()
	}
	if len(wq.pending) == 0 {
		return 0, false
	}
	idx := wq.pending[0]
	if wq.total > endgameThreshold && wq.total-wq.completed <= endgameThreshold {
		// Endgame: rotate so other peers also attempt this piece.
		wq.pending = append(wq.pending[1:], idx)
	} else {
		wq.pending = wq.pending[1:]
	}
	return idx, true
}

// Requeue returns a piece to the back of the queue. Safe to call on a piece
// that is already pending (no-op) or already completed (no-op).
func (wq *WorkQueue) Requeue(index int) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	if wq.closed || hasPiece(wq.downloaded, index) {
		return
	}
	if slices.Contains(wq.pending, index) {
		return
	}
	wq.pending = append(wq.pending, index)
	wq.cond.Signal()
}

// Done marks index as successfully downloaded. Idempotent — safe to call from
// multiple goroutines racing in endgame mode.
func (wq *WorkQueue) Done(index int) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	if hasPiece(wq.downloaded, index) {
		return // already completed by another peer
	}
	setBit(wq.downloaded, index)
	wq.completed++
	// Remove from pending if still there (endgame).
	for i, v := range wq.pending {
		if v == index {
			wq.pending = append(wq.pending[:i], wq.pending[i+1:]...)
			break
		}
	}
	if wq.completed == wq.total {
		wq.closed = true
		wq.cond.Broadcast()
	}
}

// HasPiece reports whether we have downloaded the given piece.
func (wq *WorkQueue) HasPiece(index int) bool {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	return hasPiece(wq.downloaded, index)
}

// GetBitfield returns a copy of our downloaded-pieces bitfield.
func (wq *WorkQueue) GetBitfield() []byte {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	bf := make([]byte, len(wq.downloaded))
	copy(bf, wq.downloaded)
	return bf
}

// IsComplete reports whether all pieces have been downloaded.
func (wq *WorkQueue) IsComplete() bool {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	return wq.closed
}

func (wq *WorkQueue) Total() int { return wq.total }

func (wq *WorkQueue) Remaining() int {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	return wq.total - wq.completed
}

func (wq *WorkQueue) Completed() int {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	return wq.completed
}
