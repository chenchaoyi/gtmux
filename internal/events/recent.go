package events

import (
	"bytes"
	"encoding/json"
	"os"
)

// Recent returns the newest matching records first without retaining the full
// journal. Filtering happens before the limit, including caller consent checks.
// File order, rather than timestamps, defines recency as it does for Read.
func Recent(sinceSecs, now int64, limit int, keep func(Record) bool) []Record {
	out := make([]Record, 0, min(max(limit, 0), 200))
	if limit <= 0 {
		return out
	}
	cutoff := int64(0)
	if sinceSecs > 0 {
		cutoff = now - sinceSecs
	}
	type generation struct {
		f    *os.File
		info os.FileInfo
	}
	var files []generation
	for _, path := range []string{Path(), rotatedPath()} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		fi, err := f.Stat()
		duplicate := false
		for _, old := range files {
			if err == nil && os.SameFile(old.info, fi) {
				duplicate = true
			}
		}
		if err != nil || duplicate {
			f.Close()
			continue
		}
		files = append(files, generation{f, fi})
		defer f.Close()
	}
	for _, file := range files {
		complete := newestLines(file.f, file.info.Size(), func(line []byte) bool {
			var r Record
			if json.Unmarshal(line, &r) != nil || (cutoff > 0 && r.Ts < cutoff) || (keep != nil && !keep(r)) {
				return true
			}
			out = append(out, r)
			return len(out) < limit
		})
		if !complete {
			break
		}
	}
	return out
}

// A backwards chunk reader keeps at most one bounded JSONL record plus a chunk.
// Oversized/malformed lines do not hide valid newer records or grow the buffer.
func newestLines(f *os.File, size int64, visit func([]byte) bool) bool {
	const chunkSize = 64 << 10
	const lineLimit = 1 << 20
	var carry []byte
	dropping := false
	for offset := size; offset > 0; {
		start := max(int64(0), offset-chunkSize)
		chunk := make([]byte, offset-start)
		n, err := f.ReadAt(chunk, start)
		if err != nil && n == 0 {
			return true
		}
		data := append(chunk[:n], carry...)
		carry = nil
		offset = start
		end := len(data)
		if dropping {
			i := bytes.LastIndexByte(data, '\n')
			if i < 0 {
				continue
			}
			end, dropping = i, false
		}
		for end > 0 {
			i := bytes.LastIndexByte(data[:end], '\n')
			if i < 0 {
				break
			}
			if end-i-1 <= lineLimit && !visit(data[i+1:end]) {
				return false
			}
			end = i
		}
		if end > lineLimit {
			dropping = true
		} else {
			carry = append([]byte(nil), data[:end]...)
		}
	}
	if !dropping && len(carry) > 0 {
		return visit(carry)
	}
	return true
}
