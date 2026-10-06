package hq

// The supervisor's memory, as something that can leave the machine.
//
// HQ's whole claim is that its memory survives a context reset: a situation board it
// rewrites, a knowledge base it curates, a LOCAL.md the operator personalised once. On
// the machine this was written for that is 6.1 MB accumulated over three months — and
// none of it was recoverable. No export, no snapshot, not a git repo, and (measured on
// that machine) no Time Machine destination and no cloud folder either. It survived
// every context reset and would not have survived one `rm -rf`.
//
// `LOCAL.md` is the sharpest edge: it is seeded ONCE and never overwritten, by design,
// so losing it does not self-heal. gtmux would simply see a file it does not own and
// leave the gap where the operator's own charter used to be.
//
// This is the cheap, certain layer: a single-file archive, and a daily snapshot of it
// kept beside the state rather than beside the data. It does not pretend to be an
// off-machine backup — that is a separate decision the operator has to make knowingly,
// because this archive carries their project detail. What it does cover is the failure
// that actually happens: a deletion, a bad command, a botched uninstall, a board HQ
// itself wrote into a corner.

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/humanize"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// SnapshotKeep is how many daily snapshots are retained.
//
// Two weeks: long enough that a loss noticed "sometime last week" is still recoverable,
// short enough that the whole history is a few tens of megabytes. Snapshots are only
// written when the memory CHANGED, so a quiet fortnight does not evict real history
// with fourteen identical copies of it.
const SnapshotKeep = 14

// snapshotDir holds the daily snapshots.
//
// Under the STATE dir, deliberately not inside the HQ home. The likeliest loss is the
// home going away — a deletion, a botched uninstall, a mistyped path — and a backup
// stored inside the thing it backs up goes with it. Different tree, same disk: this
// layer is about accidents, not about the disk failing.
func snapshotDir() string { return filepath.Join(state.Dir(), "hq-snapshots") }

// MemoryRoot is the directory the archive covers: HQ's home.
func MemoryRoot() string { return state.HQHome() }

// ExportMemory writes the HQ home to `dst` as a gzipped tar, returning the bytes written.
//
// One file, so it can be handed to anything: a USB stick, a synced folder, AirDrop, or
// the phone. No format of gtmux's own invention — a tarball opens on any machine, with
// or without gtmux, which matters for the one case this exists for.
func ExportMemory(dst string) (int64, error) {
	root := MemoryRoot()
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return 0, fmt.Errorf("no HQ records at %s", root)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	// Write to a temp beside the target and rename: a reader must never open a
	// half-written archive and believe it is a backup.
	f, err := os.CreateTemp(filepath.Dir(dst), ".hq-export-*")
	if err != nil {
		return 0, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	err = knowledge.WithStorageLock(func() error { return writeArchive(f, root) })
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	st, err := os.Stat(dst)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func writeArchive(w io.Writer, root string) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		// Symlinks are not followed: the home is documents, and a link out of it
		// would either duplicate something huge or dangle on restore.
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if !fi.IsDir() && !fi.Mode().IsRegular() {
			return fmt.Errorf("unsupported HQ file type: %s", rel)
		}
		h, herr := tar.FileInfoHeader(fi, "")
		if herr != nil {
			return herr
		}
		h.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		src, oerr := os.Open(p)
		if oerr != nil {
			return oerr
		}
		defer src.Close()
		_, cerr := io.Copy(tw, src)
		return cerr
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// ImportMemory restores an archive over the HQ home.
//
// An existing home is MOVED ASIDE, never overwritten in place. Restoring is done in a
// hurry and usually onto the wrong assumption; the one thing this must never do is turn
// "I restored last week's board" into "and I destroyed today's". The displaced home's
// path is returned so the caller can say where it went.
func ImportMemory(src string) (string, error) {
	archive, err := readMemoryArchive(src, "")
	if err != nil {
		return "", err
	}
	return restoreMemoryArchive(archive)
}

func restoreMemoryArchive(archive memoryArchive) (moved string, err error) {
	root := MemoryRoot()
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return "", err
	}
	prepared, err := os.MkdirTemp(filepath.Dir(root), ".hq-restore-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(prepared)
	for name, file := range archive.files {
		dst := filepath.Join(prepared, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, file.data, file.mode|0600); err != nil {
			return "", err
		}
	}
	err = withMigrationApplyLock(func() error {
		if err := memoryWriteAllowed(); err != nil {
			return err
		}
		return knowledge.WithStorageLock(func() error {
			if _, err := os.Lstat(root); err == nil {
				moved = root + ".replaced-" + filepath.Base(prepared)
				if err := os.Rename(root, moved); err != nil {
					return err
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(prepared, root); err != nil {
				if moved != "" {
					if rollback := os.Rename(moved, root); rollback != nil {
						return fmt.Errorf("restore failed: %v; rollback failed: %v; existing records remain at %s", err, rollback, moved)
					}
				}
				return err
			}
			return nil
		})
	})
	if err != nil {
		return moved, err
	}

	return moved, nil
}

// safeName rejects a path that would escape the destination. A tar can name
// `../../.ssh/authorized_keys`, and this one is unpacked from a file a user was handed.
func safeName(name string) error {
	clean := filepath.Clean("/" + filepath.ToSlash(name))
	if strings.Contains(name, "..") || filepath.IsAbs(name) || clean == "/" {
		return fmt.Errorf("archive contains an unsafe path: %q", name)
	}
	return nil
}

// MemoryFingerprint is a content hash of the HQ home, used to skip a snapshot when
// nothing changed. Names, sizes and mtimes — not contents, which would mean reading
// megabytes on a tick that runs all day.
func MemoryFingerprint() string {
	h := sha256.New()
	_ = filepath.Walk(MemoryRoot(), func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil //nolint:nilerr // an unreadable file simply does not contribute
		}
		rel, _ := filepath.Rel(MemoryRoot(), p)
		fmt.Fprintf(h, "%s|%d|%d\n", filepath.ToSlash(rel), fi.Size(), fi.ModTime().Unix())
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Snapshot writes today's snapshot unless one already exists for today with the same
// fingerprint, then prunes to SnapshotKeep. It reports whether it wrote one.
//
// Named by DATE and fingerprint, so a day's repeated ticks reuse one file and a day
// where the memory actually changed replaces it with the newer state.
func Snapshot(now time.Time) (path string, wrote bool, err error) {
	root := MemoryRoot()
	if st, serr := os.Stat(root); serr != nil || !st.IsDir() {
		return "", false, nil // no supervisor on this machine: nothing to protect
	}
	fp := MemoryFingerprint()
	day := now.Format("2006-01-02")
	path = filepath.Join(snapshotDir(), fmt.Sprintf("hq-%s-%s.tar.gz", day, fp))
	if _, err := os.Stat(path); err == nil {
		return path, false, nil // this exact memory is already snapshotted today
	}
	// Today's earlier snapshot, if the memory has moved on since, is replaced rather
	// than kept: a day is the unit, and fourteen of them is the promise.
	for _, old := range snapshotsForDay(day) {
		_ = os.Remove(old)
	}
	if _, err = ExportMemory(path); err != nil {
		return "", false, err
	}
	pruneSnapshots()
	return path, true, nil
}

func snapshotsForDay(day string) []string {
	m, _ := filepath.Glob(filepath.Join(snapshotDir(), "hq-"+day+"-*.tar.gz"))
	return m
}

// Snapshots lists the snapshots, newest first.
func Snapshots() []string {
	m, _ := filepath.Glob(filepath.Join(snapshotDir(), "hq-*.tar.gz"))
	sort.Sort(sort.Reverse(sort.StringSlice(m)))
	return m
}

func pruneSnapshots() {
	all := Snapshots()
	for i := SnapshotKeep; i < len(all); i++ {
		_ = os.Remove(all[i])
	}
}

// MemoryState is what `doctor` reports: the size of what is at risk, and what is
// actually protecting it.
type MemoryState struct {
	Exists     bool
	Bytes      int64
	Files      int
	OldestUnix int64  // when the earliest surviving file was written
	LastSnap   string // newest snapshot path, "" when there is none
	LastSnapAt int64
	Snapshots  int
}

// ReadMemoryState measures the home and the snapshots beside it.
func ReadMemoryState() MemoryState {
	var s MemoryState
	root := MemoryRoot()
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return s
	}
	s.Exists = true
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil //nolint:nilerr
		}
		s.Files++
		s.Bytes += fi.Size()
		if t := fi.ModTime().Unix(); s.OldestUnix == 0 || t < s.OldestUnix {
			s.OldestUnix = t
		}
		return nil
	})
	snaps := Snapshots()
	s.Snapshots = len(snaps)
	if len(snaps) > 0 {
		s.LastSnap = snaps[0]
		if st, err := os.Stat(snaps[0]); err == nil {
			s.LastSnapAt = st.ModTime().Unix()
		}
	}
	return s
}

// WriteMemoryArchive streams the memory to w, for a caller that is not writing a file —
// the serve endpoint hands it the HTTP response directly rather than buffering megabytes
// in a daemon whose day job is serving rows of JSON.
func WriteMemoryArchive(w io.Writer) (int64, error) {
	root := MemoryRoot()
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return 0, fmt.Errorf("no HQ records at %s", root)
	}
	c := &countingWriter{w: w}
	if err := writeArchive(c, root); err != nil {
		return c.n, err
	}
	return c.n, nil
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// timeMachineDestinations is `tmutil destinationinfo`, swappable so a test reads a machine it
// describes rather than the one it happens to run on.
var timeMachineDestinations = func() ([]byte, error) {
	return exec.Command("tmutil", "destinationinfo").Output()
}

// OffMachineHint says, plainly, whether anything carries this memory off the disk.
//
// gtmux is not a backup product and will not pretend to be one; what it can do is refuse
// to let the operator assume a protection that is not there. The machine this was written
// for had no Time Machine destination and no cloud folder, and nothing said so.
//
// It lives here rather than in the doctor row because two surfaces ask now — the row and
// the menu bar's reader — and the sentence must be the same one in both.
func OffMachineHint() string {
	if out, err := timeMachineDestinations(); err == nil &&
		!strings.Contains(string(out), "No destinations") {
		return i18n.Tr("Time Machine is configured", "已配置 Time Machine")
	}
	home := state.Home()
	for _, d := range []string{
		"Library/Mobile Documents/com~apple~CloudDocs", "Dropbox", "OneDrive", "Google Drive",
	} {
		if st, err := os.Stat(filepath.Join(home, d)); err == nil && st.IsDir() {
			return i18n.Tr("a synced folder exists; put an export there", "有同步盘，导一份过去")
		}
	}
	return i18n.Tr("nothing carries it off this disk", "没有任何东西把它带离这块盘")
}

// memoryJSON is the shape `gtmux hq --memory --json` prints — what a surface needs to say
// the same sentence the doctor row says, without re-deriving any of it.
type memoryJSON struct {
	Exists     bool   `json:"exists"`
	Bytes      int64  `json:"bytes"`
	Files      int    `json:"files"`
	Snapshots  int    `json:"snapshots"`
	LastSnapAt int64  `json:"last_snapshot_at,omitempty"`
	OldestUnix int64  `json:"oldest_at,omitempty"`
	OffMachine string `json:"off_machine"`
	Root       string `json:"root"`
	// The last export on this machine: when, and whether it was locked. Absent when
	// there has never been one — a surface must not invent a reassuring date.
	LastExportAt        int64 `json:"last_export_at,omitempty"`
	LastExportEncrypted bool  `json:"last_export_encrypted,omitempty"`
}

// printMemoryState reports what is at risk and what protects it.
func printMemoryState(asJSON bool) int {
	st := ReadMemoryState()
	if asJSON {
		row := memoryJSON{
			Exists: st.Exists, Bytes: st.Bytes, Files: st.Files, Snapshots: st.Snapshots,
			LastSnapAt: st.LastSnapAt, OldestUnix: st.OldestUnix,
			OffMachine: OffMachineHint(), Root: MemoryRoot(),
		}
		if rec, ok := LastExport(); ok {
			row.LastExportAt, row.LastExportEncrypted = rec.At, rec.Encrypted
		}
		b, err := json.Marshal(row)
		if err != nil {
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	if !st.Exists {
		i18n.Say("no HQ records on this machine", "这台机器上没有 HQ 档案")
		return 0
	}
	i18n.Say(fmt.Sprintf("%s · %d files · %d local snapshots · %s%s",
		humanBytes(st.Bytes), st.Files, st.Snapshots, OffMachineHint(), lastExportText()),
		fmt.Sprintf("%s · %d 个文件 · %d 份本地快照 · %s%s",
			humanBytes(st.Bytes), st.Files, st.Snapshots, OffMachineHint(), lastExportText()))
	return 0
}

// lastExportText is the " · last export 3d ago (locked)" tail of the memory line, or
// nothing. gtmux does not copy the records off this disk; the commander can, by exporting
// them and taking the file away, and the line says when they last exported. The record
// shows an export was written, not that the file has left the machine.
func lastExportText() string {
	rec, ok := LastExport()
	if !ok {
		return ""
	}
	secs := time.Now().Unix() - rec.At
	// "just now" is a moment, not a distance: no "ago" after it.
	when := i18n.Tr(humanize.AgeShort(secs)+" ago", humanize.AgeShort(secs)+"前")
	if secs < 60 {
		when = i18n.Tr("just now", "刚刚")
	}
	if rec.Encrypted {
		return i18n.Tr(" · last export "+when+" (locked)", " · 上次导出 "+when+"（已上锁）")
	}
	return i18n.Tr(" · last export "+when+" (not locked)", " · 上次导出 "+when+"（未上锁）")
}
