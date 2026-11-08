package transcript

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// FirstPromptRecords finds source identities, never sender identities. The caller
// supplies independently verified delivery text and its earliest possible time.
// Full history is streamed once so paging cannot make a later human copy the
// first match. Only requested prompts and opaque record IDs enter the bounded cache.
func FirstPromptRecords(ctx context.Context, agent, session string, prompts map[string]int64) map[string]string {
	if ctx.Err() != nil || normalizeAgent(agent) != "codex" || len(prompts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(prompts))
	for p := range prompts {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00", agent, session)
	for _, p := range keys {
		fmt.Fprintf(h, "%d:%d:%s", prompts[p], len(p), p)
	}
	key := fmt.Sprintf("%x", h.Sum(nil))
	paths := historyPaths(agent, session)
	files := make([]os.FileInfo, len(paths))
	for i, p := range paths {
		fi, e := os.Stat(p)
		if e != nil {
			return nil
		}
		files[i] = fi
	}
	promptWitnessCache.Lock()
	cached, ok := promptWitnessCache.entries[key]
	promptWitnessCache.Unlock()
	if ok && validPromptWitness(cached, paths, files, len(prompts)) {
		return clonePromptRecords(cached.records)
	}
	records := map[string]string{}
	for _, path := range paths {
		err := scanHistory(ctx, path, agent, session, 0, func(t Turn) bool {
			lower, wanted := prompts[t.Prompt]
			if wanted && records[t.Prompt] == "" && t.RecordID != "" {
				at, e := time.Parse(time.RFC3339Nano, t.Time)
				if e == nil && at.Unix() >= lower {
					records[t.Prompt] = t.RecordID
				}
			}
			return len(records) < len(prompts)
		})
		if err != nil {
			return nil
		}
		if len(records) == len(prompts) {
			break
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	// A concurrent replacement/rewrite cannot establish a witness for the snapshot
	// from which this scan began. Appends are safe: an existing first match cannot move.
	entry := promptWitnessEntry{paths: paths, files: files, records: records}
	current := make([]os.FileInfo, len(paths))
	for i, p := range paths {
		fi, e := os.Stat(p)
		if e != nil {
			return nil
		}
		current[i] = fi
	}
	if !validPromptWitness(entry, historyPaths(agent, session), current, len(prompts)) {
		return nil
	}
	promptWitnessCache.Lock()
	if promptWitnessCache.entries == nil {
		promptWitnessCache.entries = map[string]promptWitnessEntry{}
	}
	if len(promptWitnessCache.entries) >= 16 {
		promptWitnessCache.entries = map[string]promptWitnessEntry{}
	}
	promptWitnessCache.entries[key] = entry
	promptWitnessCache.Unlock()
	return clonePromptRecords(records)
}

type promptWitnessEntry struct {
	paths   []string
	files   []os.FileInfo
	records map[string]string
}

var promptWitnessCache struct {
	sync.Mutex
	entries map[string]promptWitnessEntry
}

func validPromptWitness(e promptWitnessEntry, paths []string, files []os.FileInfo, wanted int) bool {
	if len(paths) == 0 || len(e.paths) != len(paths) || len(files) != len(paths) {
		return false
	}
	for i, p := range paths {
		if p != e.paths[i] || !os.SameFile(e.files[i], files[i]) || files[i].Size() < e.files[i].Size() {
			return false
		}
		if files[i].Size() == e.files[i].Size() {
			if !files[i].ModTime().Equal(e.files[i].ModTime()) {
				return false
			}
		} else if len(e.records) < wanted {
			return false
		}
	}
	return true
}
func clonePromptRecords(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[strings.Clone(k)] = v
	}
	return out
}
