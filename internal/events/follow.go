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
func Follow(sinceSecs, now int64, emit func(Record), stop <-chan struct{}) {
	if sinceSecs > 0 {
		for _, r := range Read(sinceSecs, now) {
			emit(r)
		}
	}

	var f *os.File
	var rd *bufio.Reader
	var ino uint64
	open := func() {
		if f != nil {
			_ = f.Close()
		}
		nf, err := os.Open(Path())
		if err != nil {
			f, rd = nil, nil
			return
		}
		// Skip to the end on the FIRST open (we already replayed via Read); on a
		// re-open after rotation, start at 0 so the fresh file's lines all emit.
		if f == nil && ino == 0 {
			_, _ = nf.Seek(0, io.SeekEnd)
		}
		f, rd = nf, bufio.NewReader(nf)
		ino = inode(nf)
	}
	open()
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
