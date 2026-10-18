package dsh

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func Home() string {
	root := os.Getenv("DSH_HOME")
	if root == "" {
		root = filepath.Join(state.Home(), ".dsh")
	}
	if p, err := filepath.Abs(root); err == nil {
		root = p
	}
	if p, err := filepath.EvalSymlinks(root); err == nil {
		root = p
	}
	return filepath.Clean(root)
}
func HomeID() string {
	sum := sha256.Sum256([]byte("dsh\x00" + Home()))
	return hex.EncodeToString(sum[:16])
}

// Session holds saved activity, not proof of current Web/TUI ownership.
type Session struct {
	Header
	Path, HomeID, Title, State, EndReason, Revision string
	ActivityAt, FirstAt                             int64
	HasMessages                                     bool
}

type catalogEntry struct {
	cursor Cursor
	value  Session
	header bool
	err    error
}
type Catalog struct {
	mu      sync.Mutex
	entries map[string]*catalogEntry
}

var Sessions = &Catalog{}
var generationName = regexp.MustCompile(`^session(?:\.v([1-9][0-9]*))?\.jsonl(\.zstd)?$`)

// selectedFile never falls back to an older generation or one of two encodings.
func selectedFile(dir string) (string, error) {
	for _, p := range []string{filepath.Dir(dir), dir} {
		fi, err := os.Lstat(p)
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			return "", ErrUnsupported
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	version, selected, ambiguous := -1, "", false
	for _, e := range entries {
		m := generationName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v := 0
		if m[1] != "" {
			v, err = strconv.Atoi(m[1])
			if err != nil {
				return "", ErrUnsupported
			}
		}
		if v > version {
			version, selected, ambiguous = v, filepath.Join(dir, e.Name()), false
		} else if v == version {
			ambiguous = true
		}
	}
	if ambiguous {
		return "", ErrAmbiguous
	}
	if selected == "" {
		return "", os.ErrNotExist
	}
	fi, err := os.Lstat(selected)
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", ErrUnsupported
	}
	if version != 4 {
		return "", ErrUnsupported
	}
	return selected, nil
}

func sessionFiles() ([]string, error) {
	dirs, err := filepath.Glob(filepath.Join(Home(), "sessions", "*", "*"))
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, dir := range dirs {
		counts[filepath.Base(dir)]++
	}
	type artifact struct {
		path string
		at   time.Time
	}
	var files []artifact
	for _, dir := range dirs {
		if counts[filepath.Base(dir)] != 1 {
			continue
		}
		p, err := selectedFile(dir)
		if err != nil {
			continue
		}
		fi, err := os.Stat(p)
		if err == nil {
			files = append(files, artifact{p, fi.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.After(files[j].at) })
	if len(files) > 128 {
		files = files[:128]
	}
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.path
	}
	return out, nil
}

func validID(id string) bool {
	return id != "" && len(id) <= 256 && !strings.ContainsAny(id, "/\\\x00*?[]") && id != "." && id != ".."
}

func segment(id string) string {
	var b strings.Builder
	for _, c := range utf16.Encode([]rune(id)) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' {
			b.WriteRune(rune(c))
		} else {
			fmt.Fprintf(&b, "~%04X", c)
		}
	}
	return b.String()
}

// List excludes subagent-only and empty sessions. One bad artifact cannot erase
// unrelated conversations; exact reads report its refusal instead of empty history.
func (x *Catalog) List() ([]Session, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	paths, err := sessionFiles()
	if err != nil {
		return nil, err
	}
	if x.entries == nil {
		x.entries = map[string]*catalogEntry{}
	}
	keep := map[string]bool{}
	seen := map[string]int{}
	var values []Session
	for _, p := range paths {
		keep[p] = true
		e := x.entries[p]
		if e == nil {
			e = &catalogEntry{}
			x.entries[p] = e
		}
		e.err = Scan(p, &e.cursor, func() { e.value = Session{Path: p, HomeID: HomeID(), State: "idle"}; e.header = false }, func(line []byte) error { return e.fold(line) })
		if e.err != nil || !e.header {
			delete(x.entries, p)
			continue
		}
		e.value.Revision = revision(e.cursor.File)
		seen[e.value.ID]++
		if e.value.Origin != "subagent" && e.value.HasMessages {
			values = append(values, e.value)
		}
	}
	for p := range x.entries {
		if !keep[p] {
			delete(x.entries, p)
		}
	}
	out := values[:0]
	for _, s := range values {
		if seen[s.ID] == 1 {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ActivityAt > out[j].ActivityAt })
	return out, nil
}

func (x *Catalog) Resolve(id string) (Session, error) {
	if !validID(id) {
		return Session{}, fmt.Errorf("invalid DSH session id")
	}
	// Exact reads include older conversations outside the discovery window. Count
	// all matching directories before opening any: a corrupt copy is still ambiguous.
	dirs, err := filepath.Glob(filepath.Join(Home(), "sessions", "*", segment(id)))
	if err != nil {
		return Session{}, err
	}
	if len(dirs) > 1 {
		return Session{}, ErrAmbiguous
	}
	if len(dirs) == 0 {
		return Session{}, os.ErrNotExist
	}
	p, err := selectedFile(dirs[0])
	if err != nil {
		return Session{}, err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.entries == nil {
		x.entries = map[string]*catalogEntry{}
	}
	e := x.entries[p]
	if e == nil {
		if len(x.entries) >= 144 {
			for k := range x.entries {
				delete(x.entries, k)
				break
			}
		}
		e = &catalogEntry{}
		x.entries[p] = e
	}
	e.err = Scan(p, &e.cursor, func() { e.value = Session{Path: p, HomeID: HomeID(), State: "idle"}; e.header = false }, e.fold)
	if e.err != nil {
		delete(x.entries, p)
		return Session{}, e.err
	}
	if !e.header {
		return Session{}, fmt.Errorf("incomplete DSH session header")
	}
	e.value.Revision = revision(e.cursor.File)
	return e.value, nil
}

func (e *catalogEntry) fold(line []byte) error {
	if len(strings.TrimSpace(string(line))) == 0 {
		return nil
	}
	if !e.header {
		var h Header
		if err := json.Unmarshal(line, &h); err != nil {
			return err
		}
		if h.Type != "session" || h.Version != 4 {
			return ErrUnsupported
		}
		if !validID(h.ID) || len(h.Cwd) > 4096 || segment(h.ID) != filepath.Base(filepath.Dir(e.value.Path)) ||
			!strings.HasPrefix(filepath.Base(e.value.Path), "session.v4.jsonl") {
			return fmt.Errorf("invalid DSH session header")
		}
		e.value.Header = h
		e.header = true
		return nil
	}
	var event Event
	if err := json.Unmarshal(line, &event); err != nil {
		return err
	}
	if event.Type == "session" {
		return fmt.Errorf("duplicate DSH session header")
	}
	if !event.AppendsHistory() {
		return nil
	}
	at := event.Time / 1000
	switch event.Type {
	case "user/message":
		var d struct {
			Source struct {
				Kind string `json:"kind"`
			} `json:"source"`
			Role string `json:"role"`
		}
		if json.Unmarshal(event.Data, &d) != nil {
			return fmt.Errorf("invalid DSH user event")
		}
		if d.Source.Kind == "user" && (d.Role == "" || d.Role == "user") {
			e.value.HasMessages = true
			e.value.ActivityAt = at
			if e.value.FirstAt == 0 {
				e.value.FirstAt = at
			}
		}
	case "turn/start":
		e.value.State = "working"
		e.value.EndReason = ""
		e.value.ActivityAt = at
	case "turn/end":
		var d struct {
			Reason struct {
				Kind string `json:"kind"`
			} `json:"reason"`
		}
		if json.Unmarshal(event.Data, &d) != nil {
			return fmt.Errorf("invalid DSH turn end")
		}
		e.value.State = "idle"
		e.value.ActivityAt = at
		switch d.Reason.Kind {
		case "completed":
			e.value.EndReason = ""
		case "aborted", "interrupted", "forked":
			e.value.EndReason = "interrupted"
		default:
			e.value.EndReason = "error"
		}
	case "assistant/message", "tool/call", "tool/result":
		e.value.ActivityAt = at
	case "session/title":
		var d struct {
			Title string `json:"title"`
		}
		if json.Unmarshal(event.Data, &d) == nil {
			if len(d.Title) > 1024 {
				d.Title = d.Title[:1024]
			}
			e.value.Title = strings.ToValidUTF8(d.Title, "")
		}
	}
	return nil
}
func revision(fi os.FileInfo) string {
	if fi == nil {
		return ""
	}
	var ino uint64
	if s, ok := fi.Sys().(*syscall.Stat_t); ok {
		ino = s.Ino
	}
	return fmt.Sprintf("%d-%d-%d", ino, fi.Size(), fi.ModTime().UnixNano())
}
func Timestamp(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano)
}
