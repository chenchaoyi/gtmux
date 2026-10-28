// Package artifact snapshots immutable conversation documents, supplied explicitly
// or through verified conversation links, never arbitrary remote filesystem reads.
package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"golang.org/x/sys/unix"
)

const MaxBytes int64 = 30 << 20
const MaxStoredBytes int64 = 240 << 20
const MaxFiles = 256

var ErrUnavailable = errors.New("document unavailable")
var ErrLimit = errors.New("document storage limit reached")
var ErrRevoked = errors.New("document was removed")
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type File struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	MediaType string `json:"media_type"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	CreatedAt int64  `json:"created_at"`
	Sequence  uint64 `json:"sequence"`
	SessionID string `json:"session_id"`
	HomeID    string `json:"home_id,omitempty"`
	Source    string `json:"source"`
}

func (f File) Reference() string { return "gtmux-file:" + f.ID }
func directory() string          { return filepath.Join(state.Dir(), "artifacts") }

func kind(name string) (string, string) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm":
		return "html", "text/html"
	case ".md", ".markdown":
		return "markdown", "text/markdown"
	case ".pdf":
		return "pdf", "application/pdf"
	case ".png":
		return "image", "image/png"
	case ".jpg", ".jpeg":
		return "image", "image/jpeg"
	case ".txt", ".log", ".json", ".csv", ".go", ".rs", ".py", ".ts", ".tsx", ".js", ".css", ".sh", ".yaml", ".yml":
		return "text", "text/plain"
	}
	return "", ""
}

func validSession(id string) bool {
	return id != "" && len(id) <= 256 && !strings.ContainsAny(id, "\x00\r\n/\\")
}

// Add is a local, explicit publication. It snapshots a single owned regular file;
// symlinks, hard links, special files and common credential locations are refused.
func Add(source, session, home string) (File, error) {
	return add(source, session, home, false, nil, "")
}

// Linked snapshots an already verified conversation link on the owner's explicit click.
func Linked(source, session, home string) (File, error) {
	return add(source, session, home, true, nil, "")
}

func add(source, session, home string, automatic bool, root *os.Root, relative string) (File, error) {
	var out File
	if !validSession(session) || len(home) > 128 || strings.ContainsAny(home, "\x00\r\n/\\") {
		return out, fmt.Errorf("valid conversation identity required")
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return out, err
	}
	name := filepath.Base(abs)
	k, mime := kind(name)
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return out, fmt.Errorf("cannot access document: %w", err)
	}
	lower := strings.ToLower(filepath.ToSlash(canonical))
	if k == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "\x00\r\n") || strings.Contains(lower, "/.ssh/") || strings.Contains(lower, "/.aws/") || strings.Contains(lower, "/.git/") || strings.Contains(lower, "/keychains/") || strings.Contains(lower, "/.config/gtmux/") {
		return out, fmt.Errorf("unsupported document format or protected location")
	}
	// The nonblocking/no-follow open cannot hang on a replaced FIFO or follow a
	// substituted leaf. The descriptor is the only source used after this point.
	var src *os.File
	if root != nil {
		src, err = root.OpenFile(relative, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	} else {
		var handle int
		handle, err = unix.Open(abs, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err == nil {
			src = os.NewFile(uintptr(handle), abs)
		}
	}
	if err != nil {
		return out, fmt.Errorf("cannot open document")
	}
	fd := int(src.Fd())
	defer src.Close()
	beforeInfo, statErr := src.Stat()
	var st unix.Stat_t
	if statErr != nil || unix.Fstat(fd, &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFREG || int(st.Uid) != os.Geteuid() || st.Nlink != 1 || st.Size <= 0 || st.Size > MaxBytes {
		return out, fmt.Errorf("an owned, nonempty regular document up to 30 MiB is required")
	}
	data, err := io.ReadAll(io.LimitReader(src, MaxBytes+1))
	if err != nil || len(data) == 0 || int64(len(data)) > MaxBytes {
		return out, fmt.Errorf("document could not be read within its size limit")
	}
	afterInfo, statErr := src.Stat()
	if statErr != nil || afterInfo.Size() != beforeInfo.Size() || !afterInfo.ModTime().Equal(beforeInfo.ModTime()) || int64(len(data)) != st.Size {
		return out, fmt.Errorf("document changed while being published; retry after saving")
	}
	sum := sha256.Sum256(data)
	version := hex.EncodeToString(sum[:])
	id := sha256.Sum256([]byte(session + "\x00" + home + "\x00" + abs + "\x00" + version))
	out = File{ID: hex.EncodeToString(id[:16]), Name: name, Kind: k, MediaType: mime, Bytes: int64(len(data)), SHA256: version, CreatedAt: time.Now().Unix(), SessionID: session, HomeID: home, Source: abs}
	if err := state.MkdirAll(directory(), state.PrivateDir); err != nil {
		return File{}, err
	}
	lock, err := state.OpenFile(filepath.Join(directory(), ".lock"), os.O_CREATE|os.O_RDWR, state.PrivateFile)
	if err != nil {
		return File{}, err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return File{}, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	revoked, err := readRevocations()
	if err != nil {
		return File{}, err
	}
	if automatic && revoked[out.ID] {
		return File{}, ErrRevoked
	}
	clearRevocation := func() error {
		if revoked[out.ID] {
			delete(revoked, out.ID)
			return writeRevocations(revoked)
		}
		return nil
	}
	if previous, content, err := Open(out.ID); err == nil {
		content.Close()
		if err := clearRevocation(); err != nil {
			return File{}, err
		}
		return previous, nil
	}
	rows, err := entries()
	if err != nil {
		return File{}, err
	}
	var total int64
	count := 0
	for _, row := range rows {
		if !strings.HasSuffix(row.Name(), ".data") {
			continue
		}
		info, e := os.Lstat(filepath.Join(directory(), row.Name()))
		if e != nil || !info.Mode().IsRegular() {
			return File{}, ErrUnavailable
		}
		count++
		total += info.Size()
		previous, e := Get(strings.TrimSuffix(row.Name(), ".data"))
		if e != nil {
			return File{}, ErrUnavailable
		}
		if previous.Sequence >= out.Sequence {
			out.Sequence = previous.Sequence + 1
		}
	}
	if out.Sequence == 0 {
		out.Sequence = 1
	}
	if count >= MaxFiles || total+out.Bytes > MaxStoredBytes {
		return File{}, ErrLimit
	}
	if err := state.WritePrivateFile(filepath.Join(directory(), out.ID+".data"), data); err != nil {
		return File{}, err
	}
	meta, err := json.Marshal(out)
	if err == nil {
		err = state.WritePrivateFile(filepath.Join(directory(), out.ID+".json"), meta)
	}
	if err != nil {
		// Only this unpublished, newly created content is eligible for rollback.
		_ = os.Remove(filepath.Join(directory(), out.ID+".data"))
		return File{}, err
	}
	if err := clearRevocation(); err != nil {
		return File{}, err
	}
	return out, nil
}

func Get(id string) (File, error) {
	var out File
	if !idPattern.MatchString(id) {
		return out, ErrUnavailable
	}
	root, err := os.OpenRoot(directory())
	if err != nil {
		return out, ErrUnavailable
	}
	defer root.Close()
	f, err := root.OpenFile(id+".json", os.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
	if err != nil {
		return out, ErrUnavailable
	}
	defer f.Close()
	if !ownedRegular(f) {
		return File{}, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(f, 16<<10))
	if err != nil || json.Unmarshal(data, &out) != nil || out.ID != id || out.Bytes <= 0 || out.Bytes > MaxBytes || !validSession(out.SessionID) {
		return File{}, ErrUnavailable
	}
	k, mime := kind(out.Name)
	if k == "" || k != out.Kind || mime != out.MediaType || !digestPattern.MatchString(out.SHA256) || filepath.Base(out.Name) != out.Name || len(out.Name) > 1024 || strings.ContainsAny(out.Name, "\x00\r\n") || !filepath.IsAbs(out.Source) || len(out.Source) > 4096 || out.CreatedAt <= 0 {
		return File{}, ErrUnavailable
	}
	return out, nil
}

// Resolve only searches the small publication registry. It never opens reference.
func Resolve(reference, session, home string) (File, error) {
	if !validSession(session) {
		return File{}, ErrUnavailable
	}
	if strings.HasPrefix(reference, "gtmux-file:") {
		f, err := Get(strings.TrimPrefix(reference, "gtmux-file:"))
		if err == nil && f.SessionID == session && f.HomeID == home {
			return f, nil
		}
		return File{}, ErrUnavailable
	}
	if !filepath.IsAbs(reference) || filepath.Clean(reference) != reference {
		return File{}, ErrUnavailable
	}
	rows, err := entries()
	if err != nil {
		return File{}, ErrUnavailable
	}
	var newest File
	for _, row := range rows {
		if !strings.HasSuffix(row.Name(), ".json") {
			continue
		}
		f, e := Get(strings.TrimSuffix(row.Name(), ".json"))
		if e == nil && f.Source == reference && f.SessionID == session && f.HomeID == home && (f.Sequence > newest.Sequence || f.Sequence == newest.Sequence && f.CreatedAt > newest.CreatedAt) {
			newest = f
		}
	}
	if newest.ID == "" {
		return File{}, ErrUnavailable
	}
	return newest, nil
}

// Open verifies registered bytes before serving them. The held descriptor is
// rewound and streamed; a path replacement cannot redirect that response.
func Open(id string) (File, *os.File, error) {
	meta, err := Get(id)
	if err != nil {
		return File{}, nil, err
	}
	root, err := os.OpenRoot(directory())
	if err != nil {
		return File{}, nil, ErrUnavailable
	}
	defer root.Close()
	f, err := root.OpenFile(id+".data", os.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
	if err != nil {
		return File{}, nil, ErrUnavailable
	}
	info, err := f.Stat()
	if err != nil || !ownedRegular(f) || !info.Mode().IsRegular() || info.Size() != meta.Bytes {
		f.Close()
		return File{}, nil, ErrUnavailable
	}
	h := sha256.New()
	_, err = io.Copy(h, io.LimitReader(f, MaxBytes+1))
	if err != nil || hex.EncodeToString(h.Sum(nil)) != meta.SHA256 {
		f.Close()
		return File{}, nil, ErrUnavailable
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return File{}, nil, ErrUnavailable
	}
	return meta, f, nil
}

// entries bounds even a corrupt registry; it never recursively scans user files.
func entries() ([]os.DirEntry, error) {
	dir, err := os.Open(directory())
	if os.IsNotExist(err) {
		return []os.DirEntry{}, nil
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	defer dir.Close()
	rows, err := dir.ReadDir(MaxFiles*2 + 4)
	if err != nil && err != io.EOF || len(rows) > MaxFiles*2+2 {
		return nil, ErrUnavailable
	}
	return rows, nil
}

func ownedRegular(f *os.File) bool {
	var st unix.Stat_t
	return unix.Fstat(int(f.Fd()), &st) == nil && st.Mode&unix.S_IFMT == unix.S_IFREG && int(st.Uid) == os.Geteuid() && st.Nlink == 1
}

// List returns published copies, never an inventory of the source filesystem.
func List() ([]File, error) {
	rows, err := entries()
	if os.IsNotExist(err) {
		return []File{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []File{}
	for _, row := range rows {
		if !strings.HasSuffix(row.Name(), ".json") {
			continue
		}
		file, err := Get(strings.TrimSuffix(row.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Sequence > files[j].Sequence })
	return files, nil
}

// Remove revokes future reads and deletes only the registered copy. Devices may
// retain a copy they already downloaded; the user's source is never touched.
func Remove(id string) error {
	if !idPattern.MatchString(id) {
		return ErrUnavailable
	}
	lock, err := state.OpenFile(filepath.Join(directory(), ".lock"), os.O_RDWR, state.PrivateFile)
	if err != nil {
		return ErrUnavailable
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	root, err := os.OpenRoot(directory())
	if err != nil {
		return ErrUnavailable
	}
	defer root.Close()
	if _, err = Get(id); err != nil {
		return err
	}
	revoked, err := readRevocations()
	if err != nil {
		return err
	}
	if len(revoked) >= 4096 && !revoked[id] {
		return ErrLimit
	}
	revoked[id] = true
	if err = writeRevocations(revoked); err != nil {
		return err
	}
	if err = root.Remove(id + ".json"); err != nil {
		return err
	}
	return root.Remove(id + ".data")
}

// Removed snapshots cannot be silently recreated by a later remote click.
func readRevocations() (map[string]bool, error) {
	out := map[string]bool{}
	root, err := os.OpenRoot(directory())
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.OpenFile(".revocations", os.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	defer file.Close()
	if !ownedRegular(file) {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, 512<<10+1))
	if err != nil || len(data) > 512<<10 || json.Unmarshal(data, &out) != nil || out == nil || len(out) > 4096 {
		return nil, ErrUnavailable
	}
	for id, removed := range out {
		if !idPattern.MatchString(id) || !removed {
			return nil, ErrUnavailable
		}
	}
	return out, nil
}
func writeRevocations(values map[string]bool) error {
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return state.WritePrivateFile(filepath.Join(directory(), ".revocations"), data)
}
