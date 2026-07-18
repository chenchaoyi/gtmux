// Local machine resource snapshot shared by the CLI (`gtmux resource`), the usage
// report, and the serve-tick evaluator. A leaf-only helper (tmux + resource).
package radar

import (
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/resource"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// livePanePIDs maps each live tmux pane id → its pane pid (the process-tree root
// for attribution). Empty when no tmux server.
func livePanePIDs() map[string]int {
	out := map[string]int{}
	if tmux.Bin == "" {
		return out
	}
	for _, line := range tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_pid}") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) == 2 {
			if pid, err := strconv.Atoi(strings.TrimSpace(f[1])); err == nil {
				out[f[0]] = pid
			}
		}
	}
	return out
}

// CurrentResource is the live snapshot (shared by the CLI, the usage report, and
// the serve-tick evaluator).
func CurrentResource() resource.Report {
	return resource.Snapshot(livePanePIDs())
}
