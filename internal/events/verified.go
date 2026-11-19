package events

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// VerifiedSequence reads the assigned watermark without folding I/O or parse
// failures into zero. A new journal has neither a counter nor any generation.
func VerifiedSequence() (int64, error) {
	if capBytes() <= 0 {
		return 0, errors.New("event journal disabled")
	}
	info, err := os.Stat(seqPath())
	if errors.Is(err, os.ErrNotExist) {
		for _, p := range []string{rotatedPath(), Path()} {
			if _, e := os.Stat(p); !errors.Is(e, os.ErrNotExist) {
				return 0, errors.New("event journal watermark unavailable")
			}
		}
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > 32 {
		return 0, errors.New("invalid event journal watermark file")
	}
	b, err := os.ReadFile(seqPath())
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("invalid event journal watermark")
	}
	return n, nil
}

// VerifiedSince requires a readable contiguous interval including the assigned
// tail. Ordinary telemetry readers can skip invalid rows; an input gate cannot.
func VerifiedSince(floor int64) ([]Record, int64, error) {
	high, err := VerifiedSequence()
	if err != nil {
		return nil, 0, err
	}
	if high == 0 {
		return nil, 0, errors.New("no retained event journal")
	}
	if floor < 0 || high < floor {
		return nil, 0, errors.New("event watermark regressed")
	}
	var rows, legacy []Record
	var latest int64
	for _, p := range []string{rotatedPath(), Path()} {
		info, e := os.Stat(p)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, 0, e
		}
		if !info.Mode().IsRegular() {
			return nil, 0, errors.New("invalid event generation file")
		}
		f, e := os.Open(p)
		if e != nil {
			return nil, 0, e
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			var r Record
			if e = json.Unmarshal(sc.Bytes(), &r); e != nil {
				break
			}
			if r.Event == "" || r.Seq < 0 {
				e = errors.New("invalid event header")
				break
			}
			latest = max(latest, r.Seq)
			if r.Seq == 0 {
				legacy = append(legacy, r)
			} else if r.Seq > floor {
				rows = append(rows, r)
			}
		}
		if e == nil {
			e = sc.Err()
		}
		f.Close()
		if e != nil {
			return nil, 0, fmt.Errorf("unreadable event generation: %w", e)
		}
	}
	rows, gap := readSinceFrom(rows, floor)
	end, e := VerifiedSequence()
	if e != nil || end != high || latest != high || gap || int64(len(rows)) != high-floor || len(rows) > 0 && (rows[0].Seq != floor+1 || rows[len(rows)-1].Seq != high) {
		return nil, 0, errors.New("event history gap or moving watermark")
	}
	return append(rows, legacy...), high, nil
}
