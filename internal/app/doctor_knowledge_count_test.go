package app

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
)

func doctorKnowledgeFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		var content []byte
		if !entry.IsDir() {
			content, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		files[rel] = fmt.Sprintf("%s:%d:%d:%x", info.Mode(), info.Size(), info.ModTime().UnixNano(), sha256.Sum256(content))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestDKC001DoctorPendingCountMatchesCaptureProjection(t *testing.T) {
	previousLanguage := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(previousLanguage) })
	const a = "{\"id\":\"a\",\"topic\":\"pitfalls\",\"key\":\"pitfalls/a\",\"lesson\":\"synthetic a\"}\n"
	const b = "{\"id\":\"b\",\"topic\":\"pitfalls\",\"key\":\"pitfalls/b\",\"lesson\":\"synthetic b\"}\n"
	const c = "{\"id\":\"c\",\"topic\":\"pitfalls\",\"key\":\"pitfalls/c\",\"lesson\":\"synthetic c\"}\n"
	const d = "{\"id\":\"d\",\"topic\":\"pitfalls\",\"key\":\"pitfalls/d\",\"lesson\":\"synthetic d\"}\n"
	const legacy = "{\"topic\":\"pitfalls\",\"key\":\"pitfalls/legacy\",\"lesson\":\"synthetic legacy\"}\n"
	const accepted = "{\"op\":\"add\",\"id\":\"pitfalls/accepted\",\"candidate_result\":\"accepted\",\"sources\":[{\"id\":\"a\"}]}\n"
	const dismissed = "{\"op\":\"dismiss\",\"id\":\"capture/dismissed\",\"candidate_result\":\"dismissed\",\"sources\":[{\"id\":\"b\"}]}\n"
	for _, language := range []string{"en", "zh"} {
		t.Run(language, func(t *testing.T) {
			i18n.SetLang(language)
			for _, test := range []struct {
				name      string
				source    string
				ledger    string
				pending   int
				readError bool
				noTopics  bool
			}{
				{"all-settled-retained", a + b, accepted + dismissed, 0, false, false},
				{"mixed-valid-settled-invalid", a + b + c + d + "not-json\n \n", accepted + dismissed + "invalid-ledger\n", 2, false, false},
				{"legacy-stays-pending", a + legacy + "not-json\n", accepted, 1, false, false},
				{"unsettled-remains", c + d, "{\"op\":\"add\",\"id\":\"pitfalls/unsettled\",\"candidate_result\":\"proposed\",\"sources\":[{\"id\":\"c\"}]}\n", 2, false, false},
				{"missing-files", "", "", 0, false, false},
				{"ledger-read-error", a, "", 0, true, false},
				{"no-topics-keeps-empty-state", c, "", 1, false, true},
			} {
				t.Run(test.name, func(t *testing.T) {
					dir := t.TempDir()
					write := func(name, data string) {
						t.Helper()
						if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if !test.noTopics {
						write("pitfalls.md", "# Synthetic topic\n")
					}
					if test.source != "" {
						write(".pending-distill.jsonl", test.source)
					}
					if test.ledger != "" {
						write(".ledger.jsonl", test.ledger)
					}
					if test.readError {
						if err := os.Mkdir(filepath.Join(dir, ".ledger.jsonl"), 0700); err != nil {
							t.Fatal(err)
						}
					}
					before := doctorKnowledgeFiles(t, dir)
					pending, err := knowledge.ReadPendingCandidatesAt(dir)
					if (err != nil) != test.readError {
						t.Fatalf("projection read error=%v, want error=%v", err, test.readError)
					}
					if !test.readError && len(pending) != test.pending {
						t.Fatalf("official capture projection=%d, want %d", len(pending), test.pending)
					}
					row := rowHQKnowledgeAt(dir)
					switch {
					case test.noTopics:
						if row.status != stInfo {
							t.Fatalf("empty knowledge state changed: %+v", row)
						}
					case test.readError:
						if row.status != stRec {
							t.Fatalf("unreadable ledger reported as a successful count: %+v", row)
						}
					default:
						want := fmt.Sprintf("; %d pending distill", test.pending)
						if language == "zh" {
							want = fmt.Sprintf("；%d 条待蒸馏", test.pending)
						}
						if row.status != stOK || !strings.Contains(row.note, want) {
							t.Fatalf("doctor count does not match pending projection; want %q, got %+v", want, row)
						}
					}
					if after := doctorKnowledgeFiles(t, dir); !reflect.DeepEqual(before, after) {
						t.Fatal("read changed retained sources, ledger, topic, modes, mtimes or directory entries")
					}
				})
			}
		})
	}
}
