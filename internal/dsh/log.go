// Package dsh reads DeepSeek Harness session artifacts without mounting a profile,
// acquiring a writer lease, running an agent or changing the original files.
package dsh

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const MaxRecordBytes = 16 << 20
const MaxFrameBytes = 32 << 20

var ErrUnsupported = errors.New("unsupported DeepSeek Harness session generation")
var ErrAmbiguous = errors.New("ambiguous DeepSeek Harness session identity")

// Event is a current v4 durable record. Content remains opaque until the owning
// consumer selects a public event; system context and replay metadata are ignored.
type Event struct {
	Type      string          `json:"type"`
	Seq       int64           `json:"seq"`
	Time      int64           `json:"time"`
	Data      json.RawMessage `json:"data"`
	SurfaceOp json.RawMessage `json:"surfaceOp"`
}

// AppendsHistory excludes model-surface replacement copies from fresh dialogue.
// Missing metadata remains compatible with log-only public events.
func (e Event) AppendsHistory() bool {
	return len(e.SurfaceOp) == 0 || bytes.Equal(bytes.TrimSpace(e.SurfaceOp), []byte(`"append"`))
}

type Header struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	ID            string `json:"id"`
	Cwd           string `json:"cwd"`
	CreatedAt     int64  `json:"createdAt"`
	ParentSession string `json:"parentSession"`
	Origin        string `json:"origin"`
}

// Cursor always stops before an incomplete frame/line. Identity changes, shrink
// and same-size rewrites reset it. Callers discard their derived state on reset.
type Cursor struct {
	File   os.FileInfo
	Offset int64
}

// Scan folds complete committed records. It buffers one bounded frame/line,
// verifies each compressed frame's checksum and leaves a torn append pending.
func Scan(file string, c *Cursor, reset func(), consume func([]byte) error) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if c.File == nil || !os.SameFile(c.File, fi) || fi.Size() < c.Offset ||
		(fi.Size() == c.Offset && !fi.ModTime().Equal(c.File.ModTime())) {
		c.Offset = 0
		reset()
	}
	c.File = fi
	if fi.Size() == c.Offset {
		return nil
	}
	if _, err := f.Seek(c.Offset, io.SeekStart); err != nil {
		return err
	}
	r := bufio.NewReaderSize(io.LimitReader(f, fi.Size()-c.Offset), 64<<10)
	if !strings.HasSuffix(file, ".zstd") {
		for {
			line, err := readLine(r)
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if err := consume(line); err != nil {
				return err
			}
			c.Offset += int64(len(line))
		}
	}
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(MaxFrameBytes))
	if err != nil {
		return err
	}
	defer decoder.Close()
	for {
		frame, err := readFrame(r)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
		plain, err := decoder.DecodeAll(frame, nil)
		if err != nil {
			return fmt.Errorf("invalid DSH frame at byte %d: %w", c.Offset, err)
		}
		if len(plain) > MaxFrameBytes || len(plain) > 0 && plain[len(plain)-1] != '\n' {
			return fmt.Errorf("invalid DSH committed frame")
		}
		for len(plain) > 0 {
			end := 0
			for end < len(plain) && plain[end] != '\n' {
				end++
			}
			if end > MaxRecordBytes {
				return fmt.Errorf("DSH record exceeds read limit")
			}
			if err := consume(plain[:end]); err != nil {
				return err
			}
			plain = plain[end+1:]
		}
		c.Offset += int64(len(frame))
	}
}

func readLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > MaxRecordBytes {
			return nil, fmt.Errorf("DSH record exceeds read limit")
		}
		line = append(line, part...)
		if err != bufio.ErrBufferFull {
			return line, err
		}
	}
}

// readFrame follows the Zstandard frame/block headers used by the official JSONL
// backend. Dictionary/skippable frames are not part of its published container.
func readFrame(r io.Reader) ([]byte, error) {
	var frame []byte
	take := func(n int) error {
		if n < 0 || len(frame)+n > MaxFrameBytes {
			return fmt.Errorf("DSH frame exceeds read limit")
		}
		start := len(frame)
		frame = append(frame, make([]byte, n)...)
		_, err := io.ReadFull(r, frame[start:])
		return err
	}
	if err := take(5); err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint32(frame[:4]) != 0xfd2fb528 {
		return nil, fmt.Errorf("invalid DSH Zstandard magic")
	}
	d := frame[4]
	if d&0x18 != 0 || d&3 != 0 {
		return nil, fmt.Errorf("unsupported DSH frame header")
	}
	single := d&0x20 != 0
	header := 0
	if !single {
		header++
	}
	flag := d >> 6
	if flag == 0 {
		if single {
			header++
		}
	} else {
		header += 1 << flag
	}
	if err := take(header); err != nil {
		return nil, err
	}
	for {
		start := len(frame)
		if err := take(3); err != nil {
			return nil, err
		}
		block := uint32(frame[start]) | uint32(frame[start+1])<<8 | uint32(frame[start+2])<<16
		kind, size := (block>>1)&3, int(block>>3)
		if kind == 3 {
			return nil, fmt.Errorf("invalid DSH Zstandard block")
		}
		if kind == 1 {
			size = 1
		}
		if err := take(size); err != nil {
			return nil, err
		}
		if block&1 != 0 {
			break
		}
	}
	if d&4 == 0 {
		return nil, fmt.Errorf("DSH compressed frame has no checksum")
	}
	if err := take(4); err != nil {
		return nil, err
	}
	return frame, nil
}
