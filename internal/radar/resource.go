// Local machine resource snapshot shared by the CLI (`gtmux resource`), the usage
// report, and the serve-tick evaluator. A leaf-only helper (tmux + resource).
package radar

import (
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
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

// PreflightResource warns (to stderr) when a machine resource is at its RED line
// before adding load (gtmux hq / new). Returns true when it warned. Never blocks.
func PreflightResource() bool {
	m := CurrentResource().Machine
	if resource.MachineTier(m) < resource.TierRed {
		return false
	}
	i18n.Sae("⚠ resource red line: "+m.Warn+" — consider reclaiming/holding before adding load.",
		"⚠ 资源红线："+m.Warn+" —— 建议先回收或暂缓,再新增负载。")
	return true
}
