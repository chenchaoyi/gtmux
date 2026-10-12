package native

import (
	"encoding/hex"
	"path/filepath"
)

func validHomeID(id string) bool {
	if id == "" {
		return true
	}
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func scopedPath(id, home string) string {
	return filepath.Join(Dir(), "scopes", home, filepath.Base(fileFor(id)))
}
func recordPath(r Record) string {
	if r.HomeID != "" {
		return scopedPath(r.SessionID, r.HomeID)
	}
	return fileFor(r.SessionID)
}
func recordCandidates(id string) []string {
	paths := []string{fileFor(id)}
	scoped, _ := filepath.Glob(filepath.Join(Dir(), "scopes", "*", filepath.Base(fileFor(id))))
	return append(paths, scoped...)
}
func observationFiles() []string {
	legacy, _ := filepath.Glob(filepath.Join(Dir(), "*.json"))
	scoped, _ := filepath.Glob(filepath.Join(Dir(), "scopes", "*", "*.json"))
	return append(legacy, scoped...)
}

// Observations returns metadata for an exact thread ID across recorded homes.
// It performs no liveness probe, pruning or conversation read.
func Observations(id string) []Record {
	var out []Record
	for _, path := range recordCandidates(id) {
		if r, ok := loadPath(path, id); ok {
			out = append(out, r)
		}
	}
	return out
}
