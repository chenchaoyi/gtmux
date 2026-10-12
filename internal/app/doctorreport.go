package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

type doctorReportRow struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Note   string `json:"note"`
}
type doctorReportSection struct {
	Title string            `json:"title"`
	Rows  []doctorReportRow `json:"rows"`
}
type doctorReport struct {
	Schema    int                   `json:"schema"`
	Version   string                `json:"version"`
	CheckedAt int64                 `json:"checked_at"`
	Sections  []doctorReportSection `json:"sections"`
	Counts    map[string]int        `json:"counts"`
}

func writeDoctorJSON(w io.Writer, sections []dsection, now time.Time) int {
	report := doctorReport{Schema: 1, Version: Version, CheckedAt: now.Unix(), Sections: []doctorReportSection{}, Counts: map[string]int{"ok": 0, "recommended": 0, "blocking": 0, "info": 0}}
	for _, section := range sections {
		group := doctorReportSection{Title: section.title, Rows: []doctorReportRow{}}
		for _, row := range section.rows {
			status := "info"
			switch row.status {
			case stOK:
				status = "ok"
			case stRec:
				status = "recommended"
			case stMiss:
				status = "blocking"
			}
			report.Counts[status]++
			group.Rows = append(group.Rows, doctorReportRow{status, row.label, row.value, row.note})
		}
		report.Sections = append(report.Sections, group)
	}
	if json.NewEncoder(w).Encode(report) != nil {
		return 2
	}
	if report.Counts["blocking"] > 0 {
		return 1
	}
	return 0
}

func installedCLIPath() string {
	if dir := os.Getenv("GTMUX_BIN_DIR"); dir != "" {
		return filepath.Join(dir, "gtmux")
	}
	path, _ := os.Executable()
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// A health finding is not an installation failure. The NEW executable owns the
// checks; no-prompt prevents the interactive doctor's repair offer after update.
func postUpdateDoctor(binary string, timeout time.Duration) {
	i18n.Say("Update installed. Checking the environment…", "更新已安装，正在检查运行环境…")
	err := runPostUpdateDoctor(binary, timeout, os.Stdout, os.Stderr)
	if err == nil {
		return
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return
	}
	i18n.Sae("Update installed, but the environment check did not finish. Run `gtmux doctor` to retry.", "更新已安装，但环境检查未完成。请运行 `gtmux doctor` 重试。")
}
func runPostUpdateDoctor(binary string, timeout time.Duration, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "doctor", "--no-prompt", "--progress")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = 2 * time.Second
	return cmd.Run()
}
