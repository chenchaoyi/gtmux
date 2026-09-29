package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectDoctorSectionsReportsBeforeEachCheck(t *testing.T) {
	var got []string
	checks := []doctorSectionCheck{
		{"first", func() []dcheck {
			if len(got) != 1 || !strings.Contains(got[0], "first") {
				t.Fatalf("first check began without its progress line: %v", got)
			}
			return []dcheck{{status: stOK, label: "a"}}
		}},
		{"second", func() []dcheck {
			if len(got) != 2 || !strings.Contains(got[1], "second") {
				t.Fatalf("second check began without its progress line: %v", got)
			}
			return []dcheck{{status: stRec, label: "b"}}
		}},
	}
	sections := collectDoctorSections(checks, func(s string) { got = append(got, s) })
	if len(sections) != 2 || sections[0].rows[0].label != "a" || sections[1].rows[0].label != "b" {
		t.Fatalf("doctor report changed while adding progress: %#v", sections)
	}
}

func TestCollectDoctorRowsReportsBeforeEachAgentProbe(t *testing.T) {
	var stages []string
	checks := []doctorRowCheck{
		{"chat binding", func() dcheck {
			if len(stages) != 1 || !strings.Contains(stages[0], "chat binding") {
				t.Fatalf("chat scan began without a progress line: %v", stages)
			}
			return dcheck{label: "chat binding"}
		}},
		{"hook traffic", func() dcheck {
			if len(stages) != 2 || !strings.Contains(stages[1], "hook traffic") {
				t.Fatalf("event scan began without a progress line: %v", stages)
			}
			return dcheck{label: "hook traffic"}
		}},
	}
	rows := collectDoctorRows(checks, func(s string) { stages = append(stages, s) })
	if len(rows) != 2 || rows[0].label != "chat binding" || rows[1].label != "hook traffic" {
		t.Fatalf("report rows changed: %#v", rows)
	}
}

func TestBrewOutdatedVersionHasDeadlineAndNoAutoUpdate(t *testing.T) {
	brew := filepath.Join(t.TempDir(), "brew")
	if err := os.WriteFile(brew, []byte("#!/bin/sh\n"+
		"[ \"$HOMEBREW_NO_AUTO_UPDATE\" = 1 ] || exit 3\n"+
		"if [ \"$3\" = slow ]; then sleep 2; fi\n"+
		"printf '{\"formulae\":[{\"current_version\":\"3.4\"}]}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	// The full race suite starts many processes at once; give the successful
	// fixture enough startup time while the separate stalled case tests the bound.
	if got := brewOutdatedVersionWithTimeout(brew, "tmux", 5*time.Second); got != "3.4" {
		t.Fatalf("version = %q, want 3.4", got)
	}
	start := time.Now()
	if got := brewOutdatedVersionWithTimeout(brew, "slow", 20*time.Millisecond); got != "" {
		t.Fatalf("timed-out probe returned %q", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timed-out probe held doctor for %s", elapsed)
	}
}
