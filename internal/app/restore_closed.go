package app

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// tmux does not report the user's intent or the pane's exit status with
// session-closed. Scope the evidence to the still-running server that emitted it.
const (
	restoreServerFormat = "#{socket_path}\t#{pid}\t#{start_time}"
	restoreClosedHook   = "session-closed[19603]"
	restoreClosedLimit  = 512
	restoreClosedBytes  = 128 << 10
)

type restoreClosedRecord struct {
	Server   string   `json:"server"`
	Snapshot string   `json:"snapshot"`
	Modified int64    `json:"modified"`
	Names    []string `json:"names"`
}

func restoreClosedPath() string {
	return state.StorePath("restore-closed-sessions.json")
}

// Keep all existing hook entries, including a custom entry in our assigned slot.
func installRestoreClosedHook(binary string) {
	if tmux.Bin == "" {
		return
	}
	hooks, err := tmux.Run("show-hooks", "-g", "session-closed")
	if err != nil {
		return
	}
	for _, line := range strings.Split(hooks, "\n") {
		if strings.HasPrefix(line, restoreClosedHook+" ") {
			return
		}
	}
	// Escape the name and socket for the shell after tmux expands the formats.
	command := shellQuote(binary) + " restore --record-closed #{q:socket_path} #{pid} #{start_time} #{q:hook_session_name}"
	_, _ = tmux.Run("set-hook", "-g", restoreClosedHook, "run-shell -b "+strconv.Quote(command))
}

func readRestoreClosed() restoreClosedRecord {
	var record restoreClosedRecord
	f, err := state.OpenPrivateRead(restoreClosedPath())
	if err != nil {
		return record
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, restoreClosedBytes+1))
	if err != nil || len(data) > restoreClosedBytes || json.Unmarshal(data, &record) != nil || len(record.Names) > restoreClosedLimit {
		return restoreClosedRecord{}
	}
	return record
}

func matchesRestoreClosed(record restoreClosedRecord, layout savedLayout, server string) bool {
	return server != "" && record.Server == server && layout.ReadError == nil && len(layout.Digest) != 0 &&
		record.Snapshot == hex.EncodeToString(layout.Digest) && record.Modified == layout.Ref.UnixNano()
}

func restoreClosedForLayout(layout savedLayout, query func(...string) (string, error)) restoreClosedRecord {
	if layout.ReadError != nil || len(layout.Panes) == 0 {
		return restoreClosedRecord{}
	}
	record := readRestoreClosed()
	if len(record.Names) == 0 {
		return restoreClosedRecord{}
	}
	server, err := query("display-message", "-p", restoreServerFormat)
	if err != nil || !matchesRestoreClosed(record, layout, server) {
		return restoreClosedRecord{}
	}
	return record
}

func omitClosedRestorePanes(layout savedLayout, names []string) savedLayout {
	if len(names) == 0 {
		return layout
	}
	layout.Panes = slices.DeleteFunc(slices.Clone(layout.Panes), func(p savedPane) bool {
		return slices.Contains(names, p.Session)
	})
	layout.ByLoc = make(map[string]savedPane, len(layout.Panes))
	for _, pane := range layout.Panes {
		if _, exists := layout.ByLoc[pane.Loc]; !exists {
			layout.ByLoc[pane.Loc] = pane
		}
	}
	return layout
}

func recordClosedRestoreSession(args []string) {
	if len(args) != 4 || !filepath.IsAbs(args[0]) || strings.ContainsAny(args[0], "\t\r\n") {
		return
	}
	socket, pid, start, name := args[0], args[1], args[2], args[3]
	epoch, err := strconv.ParseInt(start, 10, 64)
	process, pidErr := strconv.Atoi(pid)
	if err != nil || epoch <= 0 || pidErr != nil || process <= 1 ||
		name == "" || len(name) > 1024 || strings.ContainsAny(name, "\x00\t\r\n") {
		return
	}
	query := func(command ...string) (string, error) {
		return tmux.Run(append([]string{"-S", socket}, command...)...)
	}
	server := strings.Join(args[:3], "\t")
	serverStillHasOtherSessions := func() bool {
		lines, err := query("list-sessions", "-F", restoreServerFormat+"\t#{session_name}")
		if err != nil || lines == "" {
			return false
		}
		for _, line := range strings.Split(lines, "\n") {
			if !strings.HasPrefix(line, server+"\t") || strings.TrimPrefix(line, server+"\t") == name {
				return false
			}
		}
		return true
	}

	save := resurrectLastSave()
	body, err := readRestoreSave(save)
	if save == "" || err != nil {
		return
	}
	layout := remoteSaveLayout(save, body)
	if layout.ReadError != nil || !layout.Ref.After(time.Unix(epoch+1, 0)) ||
		!slices.ContainsFunc(layout.Panes, func(p savedPane) bool { return p.Session == name }) ||
		linkedArchiveSession(body, name) {
		return
	}
	// Concurrent exits share one bounded record. Wait for the preceding atomic
	// write rather than dropping the second closure.
	lease, err := state.TryLease("restore-closed-sessions")
	for retries := 0; errors.Is(err, state.ErrLeaseHeld) && retries < 10; retries++ {
		time.Sleep(20 * time.Millisecond)
		lease, err = state.TryLease("restore-closed-sessions")
	}
	if err != nil {
		return
	}
	defer lease.Close()
	if !serverStillHasOtherSessions() || resurrectLastSave() != save {
		return
	}
	current := loadSavedLayout(save)
	if !bytes.Equal(current.Digest, layout.Digest) || !current.Ref.Equal(layout.Ref) || current.ReadError != nil {
		return
	}
	if !serverStillHasOtherSessions() {
		return
	}
	record := readRestoreClosed()
	if !matchesRestoreClosed(record, layout, server) {
		record = restoreClosedRecord{Server: server, Snapshot: hex.EncodeToString(layout.Digest), Modified: layout.Ref.UnixNano()}
	}
	if slices.Contains(record.Names, name) || len(record.Names) == restoreClosedLimit {
		return
	}
	record.Names = append(record.Names, name)
	data, err := json.Marshal(record)
	if err == nil && len(data) <= restoreClosedBytes {
		_ = state.WritePrivateFile(restoreClosedPath(), data)
	}
}

func linkedArchiveSession(body []byte, name string) bool {
	for _, line := range bytes.Split(body, []byte("\n")) {
		if !bytes.HasPrefix(line, []byte("grouped_session\t")) {
			continue
		}
		fields := strings.SplitN(string(line), "\t", 4)
		if len(fields) < 3 || fields[1] == name || fields[2] == name {
			return true
		}
	}
	return false
}
