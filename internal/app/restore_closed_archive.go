package app

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Give only this invocation of tmux-resurrect a filtered last file. Changing
// the user's global @resurrect-dir would race manual and periodic saves.
func stageClosedRestore(script string, layout savedLayout, closed restoreClosedRecord) (string, error) {
	socket, _, _ := strings.Cut(closed.Server, "\t")
	server, err := tmux.Run("-N", "-S", socket, "display-message", "-p", restoreServerFormat)
	if err != nil || !matchesRestoreClosed(closed, layout, server) {
		return "", errors.New("the tmux server for the closed sessions is unavailable")
	}
	helpers, err := os.ReadFile(filepath.Join(filepath.Dir(script), "helpers.sh"))
	if err != nil || !bytes.Contains(helpers, []byte(`tmux show-option -gqv "$option"`)) {
		return "", errors.New("tmux-resurrect archive selection is unavailable")
	}
	body, err := readRestoreSave(layout.SavePath)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	if !bytes.Equal(digest[:], layout.Digest) {
		return "", errors.New("restore archive changed")
	}
	dir, err := os.MkdirTemp("", "gtmux-restore-")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()
	if strings.ContainsAny(dir, " \t\n\r") {
		return "", errors.New("tmux-resurrect requires a temporary path without whitespace")
	}
	var filtered bytes.Buffer
	for _, row := range bytes.SplitAfter(body, []byte("\n")) {
		fields := strings.SplitN(string(row), "\t", 4)
		if len(fields) >= 2 && skipClosedArchiveLine(fields, closed.Names) {
			continue
		}
		filtered.Write(row)
	}
	if err := os.WriteFile(filepath.Join(dir, "last"), filtered.Bytes(), 0600); err != nil {
		return "", err
	}
	// Preserve the user's scrollback setting and reuse the original, read-only
	// contents archive by reference. Extraction is private to the staged restore.
	contents := filepath.Join(filepath.Dir(layout.SavePath), "pane_contents.tar.gz")
	if st, err := os.Stat(contents); err == nil && st.Mode().IsRegular() {
		if err := os.Symlink(contents, filepath.Join(dir, "pane_contents.tar.gz")); err != nil {
			return "", err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	// Recheck the server before each plugin call and forbid implicit server
	// creation, including the plugin's new-session commands.
	// Resurrect otherwise interprets a one-pane server as a new workspace and
	// can replace its surviving pane. Override that option for this call only.
	client := shellQuote(tmux.Bin) + " -N -S " + shellQuote(socket)
	wrapper := "#!/bin/sh\n" +
		"server=$(" + client + " display-message -p " + shellQuote(restoreServerFormat) + ") || exit 1\n" +
		"[ \"$server\" = " + shellQuote(closed.Server) + " ] || exit 1\n" +
		"if [ \"$#\" = 3 ] && [ \"$1\" = show-option ] && [ \"$2\" = -gqv ]; then\n" +
		"  case \"$3\" in\n" +
		"    @resurrect-dir) printf '%s\\n' " + shellQuote(dir) + "; exit 0 ;;\n" +
		"    @resurrect-never-overwrite) printf 'on\\n'; exit 0 ;;\n" +
		"  esac\nfi\n" +
		"exec " + client + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte(wrapper), 0700); err != nil {
		return "", err
	}
	ok = true
	return dir, nil
}

func skipClosedArchiveLine(fields, names []string) bool {
	switch fields[0] {
	case "pane", "window":
		return slices.Contains(names, fields[1])
	case "state":
		// A switch-client to any closed name is stale; existing client selection
		// can remain in place when the recorded active or alternate session closed.
		return slices.ContainsFunc(fields[1:], func(name string) bool {
			return slices.Contains(names, strings.TrimSpace(name))
		})
	default:
		return false
	}
}
