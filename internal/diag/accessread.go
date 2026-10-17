package diag

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

var ErrAccessFilter = errors.New("invalid access-history filter")

var ErrAccessCursor = errors.New("invalid or expired access-history cursor")

const accessReadBudget = 4 << 20

type AccessQuery struct {
	Before, Actor                         string
	DeviceID, Operation, Outcome, GroupBy string
	Limit                                 int
	Since                                 time.Time
}
type AccessPage struct {
	Entries    []AccessEntry     `json:"entries"`
	Next       string            `json:"next,omitempty"`
	Partial    bool              `json:"partial,omitempty"`
	RetainDays int               `json:"retain_days"`
	Groups     []AccessGroup     `json:"groups"`
	Query      AccessFilter      `json:"query"`
	Operations []AccessOperation `json:"operations"`
}
type accessCursor struct {
	File   string `json:"file"`
	Offset int64  `json:"offset"`
	Query  string `json:"query,omitempty"`
}

func encodeAccessCursor(file string, offset int64, query ...string) string {
	key := ""
	if len(query) > 0 {
		key = query[0]
	}
	data, _ := json.Marshal(accessCursor{file, offset, key})
	return base64.RawURLEncoding.EncodeToString(data)
}

// ReadAccess scans at most 4 MiB per page, newest first. Cursors carry only a
// validated log basename/offset; no remote input can choose a filesystem path.
// A page can be empty with Next set when ordinary diagnostics fill the scan budget.
func ReadAccess(q AccessQuery) (AccessPage, error) {
	lim := LoadLimits()
	filter := AccessFilter{Actor: q.Actor, DeviceID: q.DeviceID, Operation: q.Operation, Outcome: q.Outcome, GroupBy: q.GroupBy}
	page := AccessPage{Entries: []AccessEntry{}, Groups: []AccessGroup{}, RetainDays: lim.RetainDays, Query: filter, Operations: AccessOperations()}
	if !filter.valid() {
		return page, ErrAccessFilter
	}
	key := filter.key()
	groups := map[string]int{}
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	if q.Since.IsZero() {
		q.Since = time.Now().AddDate(0, 0, -lim.RetainDays)
	}
	dir := state.LogsDir()
	if err := state.CheckPrivateDirectory(dir); err != nil {
		if os.IsNotExist(err) {
			return page, nil
		}
		return page, err
	}
	files := Files(dir)
	index := len(files) - 1
	end := int64(-1)
	if q.Before != "" {
		if len(q.Before) > 1024 {
			return page, ErrAccessCursor
		}
		raw, err := base64.RawURLEncoding.DecodeString(q.Before)
		var cursor accessCursor
		if err != nil || json.Unmarshal(raw, &cursor) != nil || !dayFileRe.MatchString(cursor.File) || cursor.Offset < 0 {
			return page, ErrAccessCursor
		}
		found := false
		for i, file := range files {
			if filepath.Base(file.Path) == cursor.File && cursor.Offset <= file.Size {
				index, end, found = i, cursor.Offset, true
				break
			}
		}
		if !found || (cursor.Query != key && (cursor.Query != "" || filter != (AccessFilter{}))) {
			return page, ErrAccessCursor
		}
	}
	budget := int64(accessReadBudget)
	for ; index >= 0 && files[index].Day >= q.Since.Format("2006-01-02"); index-- {
		sf := files[index]
		if end < 0 {
			end = sf.Size
		}
		start := max(int64(0), end-budget)
		f, err := state.OpenPrivateRead(sf.Path)
		if err != nil {
			return page, fmt.Errorf("access history: %w", err)
		}
		data, err := io.ReadAll(io.NewSectionReader(f, start, end-start))
		_ = f.Close()
		if err != nil {
			return page, err
		}
		budget -= int64(len(data))
		// Preserve the cut line for the next page rather than silently dropping it.
		if start > 0 {
			if newline := bytes.IndexByte(data, '\n'); newline >= 0 {
				start += int64(newline + 1)
				data = data[newline+1:]
			} else {
				data = nil
			}
		}
		pos := len(data)
		for pos > 0 {
			if data[pos-1] == '\n' {
				pos--
			}
			lineStart := bytes.LastIndexByte(data[:pos], '\n') + 1
			line := data[lineStart:pos]
			pos = lineStart
			if !bytes.Contains(line, []byte("access.request")) {
				continue
			}
			var record Entry
			if json.Unmarshal(line, &record) != nil {
				continue
			}
			e, valid := accessFromEntry(record)
			if !valid || record.Time().Before(q.Since) || !filter.matches(e) {
				continue
			}
			page.Groups = addAccessGroup(page.Groups, groups, e)
			if q.GroupBy == "" {
				page.Entries = append(page.Entries, e)
			}
			if (q.GroupBy == "" && len(page.Entries) == q.Limit) || (q.GroupBy == "device" && len(page.Groups) == q.Limit) {
				offset := start + int64(lineStart)
				if offset > 0 {
					page.Next = encodeAccessCursor(filepath.Base(sf.Path), offset, key)
				} else if index > 0 {
					page.Next = encodeAccessCursor(filepath.Base(files[index-1].Path), files[index-1].Size, key)
				}
				return page, nil
			}
		}
		if start > 0 {
			page.Next = encodeAccessCursor(filepath.Base(sf.Path), start, key)
			page.Partial = true
			return page, nil
		}
		end = -1
		if budget <= 0 && index > 0 {
			page.Next = encodeAccessCursor(filepath.Base(files[index-1].Path), files[index-1].Size, key)
			page.Partial = true
			return page, nil
		}
	}
	return page, nil
}
