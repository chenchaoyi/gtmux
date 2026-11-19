package hqworkspace

import (
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// RequestGoalCheck only touches a lightweight marker for an existing workspace.
// Stop never scans records, runs a model or submits another task.
func RequestGoalCheck(home string) error {
	dir := filepath.Join(home, "workspace")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	// Touch only creates the file. Each producer edge must change its mtime.
	return state.WriteMarker(filepath.Join(dir, ".goal-check"), "check\n")
}
func GoalCheckStamp(home string) int64 {
	fi, err := os.Stat(filepath.Join(home, "workspace", ".goal-check"))
	if err != nil {
		return 0
	}
	return fi.ModTime().UnixNano()
}
