package hq

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

type SnapshotPolicy struct {
	Enabled         bool     `json:"enabled"`
	Keep            int      `json:"keep"`
	MaxMB           int      `json:"maxMB"`
	IntervalMinutes int      `json:"intervalMinutes"`
	Include         []string `json:"include,omitempty"`
}

func DefaultSnapshotPolicy() SnapshotPolicy {
	return SnapshotPolicy{Enabled: true, Keep: SnapshotKeep, MaxMB: 64, IntervalMinutes: 15}
}

func SnapshotConfiguration() (SnapshotPolicy, error) {
	c := struct {
		Snapshots SnapshotPolicy `json:"hqSnapshots"`
	}{DefaultSnapshotPolicy()}
	if err := usercfg.Load(&c); err != nil {
		return c.Snapshots, err
	}
	return c.Snapshots, ValidateSnapshotPolicy(c.Snapshots)
}
func ValidateSnapshotPolicy(c SnapshotPolicy) error {
	if c.Keep < 1 || c.Keep > 30 || c.MaxMB < 1 || c.MaxMB > 2048 || c.IntervalMinutes < 0 || c.IntervalMinutes > 1440 {
		return errors.New("snapshot keep must be 1–30, maxMB 1–2048 and intervalMinutes 0–1440")
	}
	for _, p := range c.Include {
		if p == "" || p == "." || strings.Contains(p, "\\") || filepath.ToSlash(filepath.Clean(p)) != p {
			return errors.New("snapshot include entries must be clean relative paths")
		}
		if err := safeName(p); err != nil {
			return err
		}
	}
	return nil
}
func automaticDir() string { return filepath.Join(snapshotDir(), "automatic-v1") }

type snapshotReceipt struct {
	Format   string `json:"format"`
	Digest   string `json:"digest"`
	SHA256   string `json:"sha256"`
	Bytes    int64  `json:"bytes"`
	At       int64  `json:"at"`
	Modified int64  `json:"modified_ns"`
}
type snapshotPoint struct {
	path         string
	receipt      snapshotReceipt
	logicalBytes int64
}
type snapshotStatus struct {
	LastAttempt int64  `json:"last_attempt"`
	LastSuccess int64  `json:"last_success"`
	LastError   string `json:"last_error,omitempty"`
}

func readSmallJSON(path string, dst any) error {
	before, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !before.Mode().IsRegular() {
		return errors.New("snapshot metadata is not a regular file")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return e
	}
	if !st.Mode().IsRegular() || !os.SameFile(before, st) || st.Size() > 8192 {
		return errors.New("invalid snapshot metadata")
	}
	b, e := io.ReadAll(io.LimitReader(f, 8193))
	if e != nil {
		return e
	}
	if len(b) > 8192 {
		return errors.New("snapshot metadata exceeds its size bound")
	}
	return json.Unmarshal(b, dst)
}
func snapshotHash(path string) (string, error) {
	st, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	if !st.Mode().IsRegular() {
		return "", errors.New("snapshot is not a regular file")
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !os.SameFile(st, actual) {
		return "", errors.New("snapshot changed")
	}
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func writeSnapshotJSON(path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	f, e := state.CreateTemp(filepath.Dir(path), ".metadata-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, e = f.Write(b); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e := os.Rename(f.Name(), path); e != nil {
		return e
	}
	return syncSnapshotDirectory(filepath.Dir(path))
}
func syncSnapshotDirectory(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}

func checkedPoint(path string) (snapshotPoint, error) {
	var r snapshotReceipt
	if e := readSmallJSON(path+".json", &r); e != nil {
		return snapshotPoint{}, e
	}
	if r.Format != "gtmux-hq-core-v1" || r.At == 0 {
		return snapshotPoint{}, errors.New("unmanaged snapshot")
	}
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() || st.Size() != r.Bytes {
		return snapshotPoint{}, errors.New("snapshot metadata mismatch")
	}
	hash, e := snapshotHash(path)
	if e != nil || hash != r.SHA256 {
		return snapshotPoint{}, errors.New("snapshot checksum mismatch")
	}
	a, e := readMemoryArchive(path, "")
	if e != nil {
		return snapshotPoint{}, e
	}
	if a.digest != r.Digest {
		return snapshotPoint{}, errors.New("snapshot content mismatch")
	}
	meta, e := os.Stat(path + ".json")
	if e != nil {
		return snapshotPoint{}, e
	}
	return snapshotPoint{path, r, st.Size() + meta.Size()}, nil
}
func managedSnapshotPoints() []snapshotPoint {
	names, _ := filepath.Glob(filepath.Join(automaticDir(), "hq-*.tar.gz"))
	var out []snapshotPoint
	for _, name := range names {
		if p, e := checkedPoint(name); e == nil {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path > out[j].path })
	return out
}

// Only this versioned store is managed. Legacy daily archives are never adopted
// or pruned merely because a new default has a smaller retention count.
func Snapshot(now time.Time) (path string, wrote bool, err error) {
	c, err := SnapshotConfiguration()
	if err != nil {
		return "", false, err
	}
	if !c.Enabled {
		return "", false, nil
	}
	if st, e := os.Stat(MemoryRoot()); e != nil || !st.IsDir() {
		return "", false, nil
	}
	err = knowledge.WithStorageLock(func() error {
		var status snapshotStatus
		_ = readSmallJSON(filepath.Join(automaticDir(), "status.json"), &status)
		if status.LastAttempt != 0 && now.Unix() >= status.LastAttempt && now.Unix()-status.LastAttempt < int64(c.IntervalMinutes*60) {
			return nil
		}
		if e := state.MkdirAll(automaticDir(), 0700); e != nil {
			return e
		}
		var e error
		path, wrote, e = publishAutomaticSnapshot(now, c)
		status.LastAttempt = now.Unix()
		status.LastError = ""
		if wrote {
			status.LastSuccess = now.Unix()
		}
		if e != nil {
			text := []rune(e.Error())
			if len(text) > 512 {
				text = append(text[:512], '…')
			}
			status.LastError = string(text)
		}
		if save := writeSnapshotJSON(filepath.Join(automaticDir(), "status.json"), status); save != nil && e == nil {
			e = save
		}
		return e
	})
	return
}
func publishAutomaticSnapshot(now time.Time, c SnapshotPolicy) (string, bool, error) {
	points := managedSnapshotPoints()
	digest, e := selectedContentDigest(MemoryRoot(), c.Include)
	if e != nil {
		return "", false, e
	}
	if len(points) > 0 && points[0].receipt.Digest == digest {
		return points[0].path, false, nil
	}
	f, e := state.CreateTemp(automaticDir(), ".pending-*")
	if e != nil {
		return "", false, e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	compressedLimit := min((int64(c.MaxMB)<<20)-8192, int64(archiveMaxBytes))
	e = writeSelectedArchiveLimit(f, MemoryRoot(), true, c.Include, compressedLimit)
	if e == nil {
		e = f.Sync()
	}
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return "", false, e
	}
	a, e := readMemoryArchive(tmp, "")
	if e != nil {
		return "", false, e
	}
	points = managedSnapshotPoints()
	if len(points) > 0 && points[0].receipt.Digest == a.digest {
		return points[0].path, false, nil
	}
	st, e := os.Stat(tmp)
	if e != nil {
		return "", false, e
	}
	hash, e := snapshotHash(tmp)
	if e != nil {
		return "", false, e
	}
	r := snapshotReceipt{Format: "gtmux-hq-core-v1", Digest: a.digest, SHA256: hash, Bytes: st.Size(), At: now.Unix(), Modified: st.ModTime().UnixNano()}
	rb, e := json.Marshal(r)
	if e != nil {
		return "", false, e
	}
	total := st.Size() + int64(len(rb)) + 8192 // reserve bounded status/metadata overhead
	entries, e := os.ReadDir(automaticDir())
	if e != nil {
		return "", false, e
	}
	for _, v := range entries {
		p := filepath.Join(automaticDir(), v.Name())
		if p == tmp {
			continue
		}
		fi, e := v.Info()
		if e != nil {
			return "", false, e
		}
		if fi.Mode().IsRegular() {
			total += fi.Size()
		}
	}
	budget := int64(c.MaxMB) << 20
	var retire []snapshotPoint
	for i := len(points) - 1; i >= 0 && (len(points)+1-len(retire) > c.Keep || total > budget); i-- {
		retire = append(retire, points[i])
		total -= points[i].logicalBytes
	}
	if total > budget {
		return "", false, errors.New("automatic recovery budget exceeded; existing recovery points preserved")
	}
	dst := filepath.Join(automaticDir(), fmt.Sprintf("hq-%s-%s.tar.gz", now.UTC().Format("2006-01-02T150405.000000000"), a.digest[:16]))
	if _, e := os.Lstat(dst); !os.IsNotExist(e) {
		return "", false, errors.New("snapshot destination already exists")
	}
	if e := os.Rename(tmp, dst); e != nil {
		return "", false, e
	}
	if e := writeSnapshotJSON(dst+".json", r); e != nil {
		return dst, false, e
	} // uncommitted package protected, never evicts an old point
	if e := syncSnapshotDirectory(automaticDir()); e != nil {
		return dst, false, e
	}
	if _, e := checkedPoint(dst); e != nil {
		return dst, false, e
	}
	for _, old := range retire {
		current, e := checkedPoint(old.path)
		if e != nil || current.receipt.SHA256 != old.receipt.SHA256 {
			return dst, true, errors.New("old recovery point changed; preserved without pruning")
		}
		if e := os.Remove(old.path); e != nil {
			return dst, true, e
		}
		if e := os.Remove(old.path + ".json"); e != nil {
			return dst, true, e
		}
	}
	return dst, true, syncSnapshotDirectory(automaticDir())
}

// This is a metadata view, not a new scan of legacy archive contents. Automatic
// points were fully validated when published; legacy recovery remains unknown.
type SnapshotOverview struct {
	Policy          SnapshotPolicy `json:"policy"`
	Directory       string         `json:"directory"`
	ManagedPoints   int            `json:"managed_points"`
	LegacyArchives  int            `json:"legacy_archives"`
	ManagedBytes    int64          `json:"managed_bytes"`
	LegacyBytes     int64          `json:"legacy_bytes"`
	LastValidatedAt int64          `json:"last_validated_at,omitempty"`
	LastError       string         `json:"last_error,omitempty"`
	Scope           string         `json:"scope"`
}

func ReadSnapshotOverview() SnapshotOverview {
	c, e := SnapshotConfiguration()
	out := SnapshotOverview{Policy: c, Directory: automaticDir(), Scope: "core-v1; full export remains explicit"}
	if e != nil {
		out.LastError = "Invalid snapshot configuration"
	}
	var status snapshotStatus
	if readSmallJSON(filepath.Join(automaticDir(), "status.json"), &status) == nil {
		out.LastValidatedAt = status.LastSuccess
		if out.LastError == "" {
			out.LastError = status.LastError
		}
	}
	legacy, _ := filepath.Glob(filepath.Join(snapshotDir(), "hq-*.tar.gz"))
	for _, p := range legacy {
		if st, e := os.Lstat(p); e == nil && st.Mode().IsRegular() {
			out.LegacyArchives++
			out.LegacyBytes += st.Size()
		}
	}
	entries, _ := os.ReadDir(automaticDir())
	for _, v := range entries {
		if strings.HasPrefix(v.Name(), ".") {
			continue
		}
		st, e := v.Info()
		if e != nil || !st.Mode().IsRegular() {
			continue
		}
		out.ManagedBytes += st.Size()
		if !strings.HasSuffix(v.Name(), ".tar.gz") {
			continue
		}
		var r snapshotReceipt
		if e := readSmallJSON(filepath.Join(automaticDir(), v.Name())+".json", &r); e == nil && r.Format == "gtmux-hq-core-v1" && r.Bytes == st.Size() && r.Modified == st.ModTime().UnixNano() && r.At > 0 && len(r.SHA256) == 64 {
			out.ManagedPoints++
		}
	}
	return out
}
