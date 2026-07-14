package hqfeed

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// FollowSpool tails the spool, calling emit for each new record, until stop is
// closed — the subscription HQ backgrounds (`gtmux hq-feed --tail`). It is
// rotation-aware (tail -F semantics): when the active spool is rotated away it
// re-opens the fresh file and keeps going. It first replays the tail from
// sinceSecs back (0 = none), then streams new appends. Poll-based (250ms), cgo-free.
func FollowSpool(sinceSecs, now int64, emit func(events.Record), stop <-chan struct{}) {
	if sinceSecs > 0 {
		for _, r := range ReadSpool(sinceSecs, now) {
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
		nf, err := os.Open(SpoolPath())
		if err != nil {
			f, rd = nil, nil
			return
		}
		if f == nil && ino == 0 {
			_, _ = nf.Seek(0, io.SeekEnd) // first open: we already replayed via ReadSpool
		}
		f, rd = nf, bufio.NewReader(nf)
		ino = spoolInode(nf)
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
					var r events.Record
					if json.Unmarshal(line[:len(line)-1], &r) == nil {
						emit(r)
					}
					continue
				}
				break
			}
		}
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if rd == nil || spoolRotated(ino) {
			open()
		}
	}
}

func spoolInode(f *os.File) uint64 {
	fi, err := f.Stat()
	if err != nil {
		return 0
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}

func spoolRotated(openInode uint64) bool {
	fi, err := os.Stat(SpoolPath())
	if err != nil {
		return true
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino) != openInode
	}
	return true
}
