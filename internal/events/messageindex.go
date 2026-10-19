package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
)

// MessageIndex retains only stable public-message keys, folding appended lines
// without decoding the rest of the lifecycle journal into Record values.
type MessageIndex struct {
	mu     sync.Mutex
	file   os.FileInfo
	offset int64
	keys   map[string]bool
	order  []string
	next   int
}

var PublicMessages = &MessageIndex{}

func (x *MessageIndex) Contains(key string) bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	fi, err := os.Stat(Path())
	if err != nil {
		x.file = nil
		x.keys = nil
		x.offset = 0
		return false
	}
	if x.file == nil || !os.SameFile(x.file, fi) || fi.Size() < x.offset || fi.Size() == x.offset && !fi.ModTime().Equal(x.file.ModTime()) {
		x.keys = map[string]bool{}
		x.order = nil
		x.next = 0
		x.offset = 0
		x.fold(rotatedPath(), 0)
	}
	x.file = fi
	if fi.Size() > x.offset {
		x.offset = x.fold(Path(), x.offset)
	}
	return x.keys[key]
}

func (x *MessageIndex) fold(path string, offset int64) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}
	r := bufio.NewReaderSize(f, 64<<10)
	var line []byte
	discard := false
	for {
		fragment, err := r.ReadSlice('\n')
		if len(line)+len(fragment) <= 2<<20 && !discard {
			line = append(line, fragment...)
		} else {
			discard = true
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return offset
		}
		offset += int64(len(line))
		if discard {
			// Oversized irrelevant records must still advance over their full
			// committed bytes; use the reader's absolute position below.
			pos, _ := f.Seek(0, io.SeekCurrent)
			offset = pos - int64(r.Buffered())
		} else if bytes.Contains(line, []byte(`"message_id"`)) {
			var rec struct {
				MessageID string `json:"message_id"`
			}
			if json.Unmarshal(line, &rec) == nil && rec.MessageID != "" && len(rec.MessageID) <= 128 {
				if x.keys == nil {
					x.keys = map[string]bool{}
				}
				if !x.keys[rec.MessageID] {
					if len(x.order) < 65536 {
						x.order = append(x.order, rec.MessageID)
					} else {
						delete(x.keys, x.order[x.next])
						x.order[x.next] = rec.MessageID
						x.next = (x.next + 1) % len(x.order)
					}
					x.keys[rec.MessageID] = true
				}
			}
		}
		line = line[:0]
		discard = false
	}
}
