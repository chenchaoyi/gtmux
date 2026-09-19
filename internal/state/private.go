package state

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Everything under gtmux's two roots is readable by its owner only.
//
// It was not. On the machine that exposed it, 88 MB under ~/.local/share/gtmux was
// readable by every account on the Mac: the serve token in 398 log lines, conversation
// summaries in the journal, error lines lifted from agent transcripts, images sent from
// the phone. The token file itself was 0600; the files around it undid that. The cause
// was not one writer but every writer: 155 call sites passed a literal 0644 or 0755.
//
// Three mechanisms, because each leaves a gap the others close:
//
//   - PrivateUmask at every entry point makes whatever gtmux creates private, including
//     a writer added next year that passes 0644 out of habit.
//   - WriteForeign keeps a conventional mode on files gtmux edits but does not own, which
//     the umask would otherwise narrow when gtmux happens to be the one creating them.
//   - Narrow re-applies the modes to both trees on a schedule, because other processes
//     write there with their own umask (the HQ agent writes its board into its home).

// Modes for gtmux's own files and directories.
const (
	PrivateFile fs.FileMode = 0o600
	PrivateDir  fs.FileMode = 0o700
)

// PrivateUmask sets the process umask so every file and directory gtmux creates is
// private, whatever mode the call site asked for. It returns the previous umask.
func PrivateUmask() int { return syscall.Umask(0o077) }

// ConfigDir is ~/.config/gtmux: what a person set, or would carry to a new machine —
// settings, credentials, HQ's home. Everything gtmux generates lives under Dir().
func ConfigDir() string { return filepath.Join(home(), ".config", "gtmux") }

// LogsDir holds the local log store: one JSON Lines file per local day.
func LogsDir() string { return filepath.Join(Dir(), "logs") }

// StatusDir holds one small JSON file per component: what is true right now.
func StatusDir() string { return filepath.Join(Dir(), "status") }

// CacheDir holds what can be deleted at any time and regrows: rendered icons.
func CacheDir() string { return filepath.Join(Dir(), "cache") }

// TunnelURLPath is where the running tunnel records its public address. It is runtime
// state, so it lives in the data root; LegacyTunnelURLPath is where it used to be, and
// readers fall back to it for one release.
func TunnelURLPath() string       { return filepath.Join(Dir(), "tunnel-url") }
func LegacyTunnelURLPath() string { return filepath.Join(ConfigDir(), "tunnel-url") }

// WriteForeign writes a file gtmux edits but does not own: an agent's settings, the
// user's instruction file, ~/.tmux.conf, a repository's AGENTS.md. An existing file keeps
// its mode (os.WriteFile rewrites in place). A new one gets perm explicitly, so the
// private umask does not make a file other tools expect to read 0600.
func WriteForeign(path string, data []byte, perm fs.FileMode) error {
	_, statErr := os.Stat(path)
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	if os.IsNotExist(statErr) {
		return os.Chmod(path, perm)
	}
	return nil
}

// A Narrowed is one mode change Narrow made, for the caller to record.
type Narrowed struct {
	Path     string
	From, To fs.FileMode
}

// Narrow removes group and other permissions from everything under root, root included,
// and keeps the owner's exactly: a 0644 file becomes 0600, a 0755 directory 0700, and a
// script HQ keeps in knowledge/tools/ stays executable for its owner. Setting a flat 0600
// would have stripped that. It does not follow symlinks, and it skips what it cannot
// change (a file the privileged server-mode guard created as root), because narrowing is
// housekeeping and must never fail the command running it. It returns what it changed.
func Narrow(root string) []Narrowed {
	var out []Narrowed
	uid := os.Getuid()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: skip it and keep walking
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != uid {
			return nil // not ours to change
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return nil // sockets, fifos
		}
		have := info.Mode().Perm()
		if have&0o077 == 0 {
			return nil
		}
		want := have & 0o700
		if os.Chmod(p, want) == nil {
			out = append(out, Narrowed{Path: p, From: have, To: want})
		}
		return nil
	})
	return out
}

// WideModes counts what under root is readable or writable by accounts other than its
// owner, without changing anything, and returns one example path. doctor reports it;
// Narrow fixes it.
func WideModes(root string) (count int, example string) {
	uid := os.Getuid()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != uid {
			return nil
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		if info.Mode().Perm()&0o077 != 0 {
			if count == 0 {
				example = p
			}
			count++
		}
		return nil
	})
	return count, example
}
