package hq

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHQPlaybookBudgetLinksAndImports(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	links := regexp.MustCompile(`\]\(([^)]+\.md)\)`)
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			if _, err := seedHQHome(lang); err != nil {
				t.Fatal(err)
			}
			entry, err := os.ReadFile(hqInstructionsPath())
			if err != nil {
				t.Fatal(err)
			}
			if lines := strings.Count(string(entry), "\n"); lines > 125 || len(entry) > 10*1024 {
				t.Fatalf("entry budget exceeded: %d lines, %d bytes", lines, len(entry))
			}
			docs := map[string]string{"AGENTS.md": string(entry)}
			found := map[string]int{}
			for _, match := range links.FindAllStringSubmatch(string(entry), -1) {
				found[match[1]]++
			}
			for _, name := range runbookNames {
				path := filepath.Join(runbookDir(lang), name)
				if found[path] != 1 {
					t.Errorf("%s: expected one trigger in the entry, found %d", path, found[path])
				}
				delete(found, path)
				body, err := os.ReadFile(filepath.Join(state.HQHome(), path))
				if err != nil || string(body) != playbookText(lang, name) {
					t.Fatalf("runbook %s is missing or incorrect: %v", path, err)
				}
				docs[name] = string(body)
			}
			if len(found) != 0 {
				t.Errorf("unresolved entry links: %v", found)
			}

			seen := map[string]string{}
			imports := regexp.MustCompile(`(^|[[:space:]])@[^[:space:]]+`)
			for name, body := range docs {
				if name != "AGENTS.md" && len(body) > 8*1024 {
					t.Errorf("runbook %s exceeds 8 KiB", name)
				}
				for _, line := range strings.Split(body, "\n") {
					plain := strings.TrimSpace(line)
					if imports.MatchString(plain) && (name != "AGENTS.md" || plain != "@LOCAL.md") {
						t.Errorf("%s contains an unplanned automatic import: %q", name, plain)
					}
					if len(plain) < 60 {
						continue
					}
					if prev, ok := seen[plain]; ok {
						t.Errorf("duplicate paragraph in %s and %s: %.80s", prev, name, plain)
					}
					seen[plain] = name
				}
			}
			if !strings.HasSuffix(string(entry), "\n@LOCAL.md\n") || strings.Count(string(entry), "@LOCAL.md") != 1 {
				t.Fatal("expected the one compatible LOCAL import at the end")
			}
		})
	}
}

func TestHQRunbooksRepairWithoutRewritingEntry(t *testing.T) {
	hqrHome(t)
	if _, err := seedHQHome("zh"); err != nil {
		t.Fatal(err)
	}
	entry, err := os.Stat(hqInstructionsPath())
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(state.HQHome(), runbookDir("zh"), "workspace.md")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	hqrRun(t, 0, "--refresh-rules")
	if got, err := os.ReadFile(missing); err != nil || string(got) != playbookText("zh", "workspace.md") {
		t.Fatalf("missing runbook was not repaired: %v", err)
	}
	if current, err := os.Stat(hqInstructionsPath()); err != nil || !os.SameFile(entry, current) || entry.ModTime() != current.ModTime() {
		t.Fatalf("repair rewrote the entry: %v", err)
	}
}

func TestHQRunbookConflictKeepsEntryAndLocal(t *testing.T) {
	hqrHome(t)
	old := playbookMarker(67, "en") + "\nold known charter\n@LOCAL.md\n"
	local := strings.Repeat("legacy policy that must be kept\n", 3500) + "unique final standing rule\n"
	conflict := filepath.Join(state.HQHome(), runbookDir("en"), "workspace.md")
	for path, body := range map[string]string{hqInstructionsPath(): old, hqLocalPath(): local, conflict: "different bytes"} {
		if err := state.WritePrivateFile(path, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	hqrRun(t, 1, "--refresh-rules")
	for path, want := range map[string]string{hqInstructionsPath(): old, hqLocalPath(): local, conflict: "different bytes"} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("failed upgrade changed %s: %v", path, err)
		}
	}
}

func TestHQPlaybookLanguageParity(t *testing.T) {
	anchors := regexp.MustCompile("(?m)^#{1,3} |\x60gtmux [a-z]+|\x60--[a-z-]+\x60")
	for _, name := range append([]string{"entry.md"}, runbookNames...) {
		bag := func(lang string) map[string]int {
			counts := map[string]int{}
			for _, token := range anchors.FindAllString(playbookText(lang, name), -1) {
				counts[token]++
			}
			return counts
		}
		en, zh := bag("en"), bag("zh")
		for key, expected := range en {
			if zh[key] != expected {
				t.Errorf("%s %q: en %d, zh %d", name, key, expected, zh[key])
			}
			delete(zh, key)
		}
		if len(zh) > 0 {
			t.Errorf("%s has zh-only anchors: %v", name, zh)
		}
	}
}

func TestHQPlaybookGatesStayInTheEntry(t *testing.T) {
	for _, rule := range []struct {
		name string
		en   string
		zh   string
	}{
		{"identity", "match this session to the HQ pane stamp", "核对本会话、HQ 窗格标记"},
		{"local", "read it through the end", "读到文件末尾"},
		{"authority", "--for user", "--for user"},
		{"unfollowed desktop", "status is passive visibility only", "只可看摘要状态"},
		{"unconfirmed delivery", "queued send", "排队、空闲"},
		{"draft", "never use `--force` to bypass", "永不使用 `--force` 越过"},
		{"low disk", "hold disk-heavy work", "暂停高写入任务"},
		{"reclaim", "obtain approval for unapproved irreversible cleanup", "范围外不可逆清理交用户批准"},
		{"goal responsibility", "check the affected open workspace goals even when unread is empty", "即使 unread 为空，也要检查触及的未完事项"},
		{"recovery", "specific safe recovery or user step", "安全恢复或用户介入步骤"},
		{"storage", "task temporary storage", "任务临时目录"},
		{"private evidence", "confirm the exact proposed text", "准确拟写内容并取得确认"},
	} {
		t.Run(rule.name, func(t *testing.T) {
			for lang, want := range map[string]string{"en": rule.en, "zh": rule.zh} {
				entry := strings.ToLower(playbookIn(lang))
				if !strings.Contains(entry, strings.ToLower(want)) {
					t.Errorf("%s entry lost its standing rule %q", lang, want)
				}
			}
		})
	}
}
