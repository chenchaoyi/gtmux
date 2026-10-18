package events

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// The assigned counter can be ahead of committed records. Keep LatestSeq's
// retained-record meaning while making unchanged calls a pair of metadata reads.
type retainedSeqIndex struct {
	mu     sync.Mutex
	files  map[string]*seqGeneration
	parsed int
}

type seqGeneration struct {
	info                 os.FileInfo
	offset, max, pending int64
}

var retainedSequences retainedSeqIndex

func (x *retainedSeqIndex) latest() int64 {
	x.mu.Lock()
	defer x.mu.Unlock()
	next := make(map[string]*seqGeneration)
	var latest int64
	for _, path := range []string{rotatedPath(), Path()} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		var e *seqGeneration
		for previous, candidate := range x.files {
			if filepath.Dir(previous) == filepath.Dir(path) && os.SameFile(candidate.info, fi) {
				e = candidate
				break
			}
		}
		if e == nil || fi.Size() < e.info.Size() || (fi.Size() == e.info.Size() && !fi.ModTime().Equal(e.info.ModTime())) {
			e = &seqGeneration{}
		}
		if e.info == nil || fi.Size() != e.info.Size() {
			x.fold(f, fi.Size(), e)
		}
		e.info = fi
		next[path] = e
		latest = max(latest, e.max, e.pending)
		f.Close()
	}
	x.files = next
	return latest
}

func (x *retainedSeqIndex) fold(f *os.File, size int64, e *seqGeneration) {
	start, consumed := e.offset, int64(0)
	e.pending = 0
	complete := false
	sc := bufio.NewScanner(io.NewSectionReader(f, start, size-start))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	sc.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := bufio.ScanLines(data, atEOF)
		if advance > 0 {
			consumed += int64(advance)
			complete = data[advance-1] == '\n'
			if complete {
				e.offset = start + consumed
			}
		}
		return advance, token, err
	})
	for sc.Scan() {
		var header struct {
			Seq int64 `json:"seq"`
		}
		if json.Unmarshal(sc.Bytes(), &header) != nil {
			continue
		}
		x.parsed++
		if complete {
			e.max = max(e.max, header.Seq)
		} else {
			// Read historically accepts valid JSON at EOF without a final newline.
			// Recheck it on append instead of committing an incomplete line offset.
			e.pending = header.Seq
		}
	}
}
