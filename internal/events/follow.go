package events

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"
)

// Follow tails the active log, calling emit for each new record, until stop is
// closed. It is ROTATION-AWARE (tail -F semantics): when the active file is
// rotated out from under it (renamed away → a fresh, smaller file appears at the
// same path), it re-opens and keeps going, so following never silently stops.
//
// It first replays the tail already on disk from `sinceSecs` back (0 = none),
// then streams new appends. Polling-based (250ms) — no fsnotify dep, cgo-free.
//
// The replay and the stream meet without a hole. The active file is opened, at its end,
// BEFORE the replay reads, so whatever is appended while the replay runs lies past that
// point and is streamed next; it used to be opened after, at the end it had by then, and
// an append that landed during the replay was never delivered (%12, 2026-10-06). A record
// both read is emitted once: the stream skips the sequence numbers the replay emitted.
// (Seq order is not file order, as a writer appends after taking its number, so this is a
// set, not a high-water mark. Only legacy records lack a seq, and they are all older than
// the open.)
func Follow(sinceSecs, now int64, emit func(Record), stop <-chan struct{}) {
	var f *os.File
	var rd *bufio.Reader
	var ino uint64
	opened := false // whether open has run once
	open := func() {
		if f != nil {
			_ = f.Close()
		}
		first := !opened
		opened = true
		nf, err := os.Open(Path())
		if err != nil {
			f, rd = nil, nil
			return
		}
		// Skip to the end on the very FIRST open: what is already there is the replay's.
		// Any later open (a file that did not exist yet, or a fresh one after rotation)
		// starts at 0, so all of that file's lines emit.
		if first {
			_, _ = nf.Seek(0, io.SeekEnd)
		}
		f, rd = nf, bufio.NewReader(nf)
		ino = inode(nf)
	}
	open()
	afterFollowOpen()

	replayed := map[int64]bool{}
	if sinceSecs > 0 {
		for _, r := range Read(sinceSecs, now) {
			if r.Seq > 0 {
				replayed[r.Seq] = true
			}
			emit(r)
		}
	}

	defer func() {
		if f != nil {
			_ = f.Close()
		}
	}()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if rd != nil {
			for {
				line, err := rd.ReadBytes('\n')
				if len(line) > 0 && err == nil {
					var r Record
					if json.Unmarshal(line[:len(line)-1], &r) == nil {
						if r.Seq > 0 && replayed[r.Seq] {
							delete(replayed, r.Seq) // the replay emitted it already
							continue
						}
						emit(r)
					}
					continue
				}
				break // EOF / partial line — wait for more
			}
		}
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		// Detect rotation: the path now points at a different inode (a fresh file
		// after our active one was renamed away), or it shrank/vanished → re-open.
		if rd == nil || rotated(ino) {
			open()
		}
	}
}

// rotated reports whether the file at Path() is no longer the one we opened
// (inode changed) or is gone.
func rotated(openInode uint64) bool {
	fi, err := os.Stat(Path())
	if err != nil {
		return true
	}
	return statInode(fi) != openInode
}

// afterFollowOpen runs between Follow's first open and its replay. A test stands in for a
// writer there; it does nothing otherwise.
var afterFollowOpen = func() {}
