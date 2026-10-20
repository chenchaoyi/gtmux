package agentprofile

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

const maxPoints = 1024
const tailBudget = 2 << 20
const recordBudget = 1 << 20

type logIndex struct {
	file    os.FileInfo
	offset  int64
	points  []point
	cursor  dsh.Cursor
	used    time.Time
	partial bool
}

var indexes = struct {
	sync.Mutex
	entries map[string]*logIndex
}{entries: map[string]*logIndex{}}

// ReadLogs indexes allowed metadata only. JSONL bootstraps from a bounded tail;
// compressed DSH uses its existing frame cursor, never an events-log scan.
func ReadLogs(p *Profile, paths []string, from, until int64) {
	indexes.Lock()
	defer indexes.Unlock()
	now := time.Now()
	for k, v := range indexes.entries {
		if now.Sub(v.used) > 10*time.Minute {
			delete(indexes.entries, k)
		}
	}
	if len(paths) > 8 {
		paths = paths[len(paths)-8:]
		p.Partial = true
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		key := p.Agent + "\x00" + p.SessionID + "\x00" + path
		v := indexes.entries[key]
		if v == nil {
			for len(indexes.entries) >= 16 {
				var oldest string
				var at time.Time
				for k, v := range indexes.entries {
					if oldest == "" || v.used.Before(at) {
						oldest, at = k, v.used
					}
				}
				delete(indexes.entries, oldest)
			}
			v = &logIndex{}
			indexes.entries[key] = v
		}
		v.used = now
		if err := v.read(p.Agent, path); err != nil {
			p.Partial = true
			p.Unavailable = true
			continue
		}
		p.Partial = p.Partial || v.partial
		apply(p, v.points, from, until)
	}
}

func (v *logIndex) consume(agent string, line []byte) {
	if len(line) > recordBudget {
		v.partial = true
		return
	}
	x := decode(agent, line)
	if len(x.Facts) == 0 && !x.End {
		return
	}
	v.points = append(v.points, x)
	if len(v.points) > maxPoints {
		v.partial = true
		v.points = append([]point(nil), v.points[len(v.points)-maxPoints:]...)
	}
}

func (v *logIndex) read(agent, path string) error {
	if strings.HasSuffix(path, ".zstd") {
		return dsh.Scan(path, &v.cursor, func() { v.points = nil; v.partial = false }, func(b []byte) error { v.consume(agent, b); return nil })
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	reset := v.file == nil || !os.SameFile(v.file, fi) || fi.Size() < v.offset || fi.Size() == v.offset && !fi.ModTime().Equal(v.file.ModTime())
	if reset {
		v.partial = false
		v.points = nil
		v.offset = 0
		// Session version/provider live in the first header, even when the tail is bounded.
		r := bufio.NewReaderSize(io.LimitReader(f, recordBudget+1), recordBudget+1)
		if line, e := r.ReadBytes('\n'); e == nil && len(line) <= recordBudget {
			v.consume(agent, line)
		}
		if fi.Size() > tailBudget {
			v.partial = true
			v.offset = fi.Size() - tailBudget
			_, _ = f.Seek(v.offset, io.SeekStart)
			r = bufio.NewReader(f)
			line, _ := r.ReadBytes('\n')
			v.offset += int64(len(line))
		}
	}
	v.file = fi
	if fi.Size()-v.offset > tailBudget {
		v.partial = true
		v.offset = fi.Size() - tailBudget
		v.points = nil
		_, _ = f.Seek(v.offset, io.SeekStart)
		r := bufio.NewReader(f)
		b, _ := r.ReadBytes('\n')
		v.offset += int64(len(b))
	}
	_, err = f.Seek(v.offset, io.SeekStart)
	if err != nil {
		return err
	}
	r := bufio.NewReaderSize(io.LimitReader(f, fi.Size()-v.offset), 64<<10)
	for {
		start := v.offset
		var line []byte
		oversized := false
		for {
			b, e := r.ReadSlice('\n')
			v.offset += int64(len(b))
			if len(line)+len(b) > recordBudget {
				oversized = true
				line = nil
			}
			if !oversized {
				line = append(line, b...)
			}
			if e == bufio.ErrBufferFull {
				continue
			}
			if e == io.EOF {
				v.offset = start
				return nil
			}
			if e != nil {
				return e
			}
			if !oversized && len(bytes.TrimSpace(line)) > 0 {
				v.consume(agent, line)
			}
			break
		}
	}
}
