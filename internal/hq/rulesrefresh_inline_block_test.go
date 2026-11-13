package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHQR011InlineInSyncKnowledgeRemainsEffective(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			home := hqrHome(t)
			for _, agent := range agents.All() {
				if agent.InstructionsEnv != "" {
					t.Setenv(agent.InstructionsEnv, filepath.Join(home, "carriers", agent.Key))
				}
			}
			t.Setenv("CODEX_HOME", state.HQHome())
			// Public sync is used only after every resolved carrier is proven inside the
			// isolated fixture. No hand-forged hash or private knowledge parser copy.
			for _, carrier := range knowledge.Carriers() {
				rel, err := filepath.Rel(home, carrier.Path)
				if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					t.Fatal("carrier escaped fixture", carrier)
				}
			}
			if _, err := knowledge.SyncMachine(false); err != nil {
				t.Fatal(err)
			}
			canonical, err := os.ReadFile(hqInstructionsPath())
			if err != nil {
				t.Fatal(err)
			}
			inline := "<!-- retained annotation -->" + strings.Replace(strings.TrimSuffix(string(canonical), "\n"), "\n<!-- gtmux:knowledge:end -->", "<!-- gtmux:knowledge:end -->", 1) + "\n"
			prior := strings.Replace(playbookIn(lang), playbookMarker(hqPlaybookVersion, lang), playbookMarker(hqPlaybookVersion-1, lang), 1)
			prior = strings.Replace(prior, "\n@LOCAL.md\n", "\n"+inline+"\n@LOCAL.md\n", 1)
			hqrPut(t, hqInstructionsPath(), prior)
			inSync := func() bool {
				states, err := knowledge.CarrierStatuses()
				if err != nil {
					t.Fatal(err)
				}
				for _, s := range states {
					if s.Agent == "codex" && s.Path == hqInstructionsPath() {
						return s.State == knowledge.SyncInSync
					}
				}
				t.Fatal("Codex fixture carrier not found")
				return false
			}
			if !inSync() {
				t.Fatal("inline fixture not accepted by actual knowledge protocol")
			}
			rep, err := knowledge.SyncMachine(false)
			if err != nil {
				t.Fatal(err)
			}
			kept := false
			for _, agent := range rep.Kept {
				kept = kept || agent == "codex"
			}
			same, _ := os.ReadFile(hqInstructionsPath())
			if !kept || string(same) != prior {
				t.Fatal("real sync did not keep valid inline bytes", rep)
			}
			before, _ := os.Stat(hqInstructionsPath())
			got := CmdHQ([]string{"--refresh-rules"})
			after, _ := os.Stat(hqInstructionsPath())
			body, _ := os.ReadFile(hqInstructionsPath())
			backups, _ := filepath.Glob(hqInstructionsPath() + ".bak-*")
			if got != 1 || string(body) != prior || !before.ModTime().Equal(after.ModTime()) || len(backups) != 0 || !inSync() {
				t.Fatalf("valid in-sync inline knowledge removed from effective rules: exit=%d same=%v backups=%d", got, string(body) == prior, len(backups))
			}
		})
	}
}
