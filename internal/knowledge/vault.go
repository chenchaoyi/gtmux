package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const vaultManifest = ".gtmux-vault.json"

type vaultEntry struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Path     string `json:"path"`
	Topic    string `json:"topic"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

type vaultBase struct {
	Version int          `json:"version"`
	Entries []vaultEntry `json:"entries"`
}

type VaultResult struct {
	Written   int             `json:"written"`
	Skipped   int             `json:"skipped"`
	Conflicts []string        `json:"conflicts"`
	Missing   []string        `json:"missing,omitempty"`
	Drafts    []string        `json:"drafts,omitempty"`
	Proposal  *ReviewProposal `json:"proposal,omitempty"`
}

func vaultRoot(path string, create bool) (string, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// Resolve existing parents before judging the storage boundary (/tmp on macOS).
	parent := root
	var tail []string
	for {
		resolved, e := filepath.EvalSymlinks(parent)
		if e == nil {
			root = filepath.Join(append([]string{resolved}, tail...)...)
			break
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", e
		}
		tail = append([]string{filepath.Base(parent)}, tail...)
		parent = next
	}
	for _, managed := range []string{state.HQHome(), state.Dir(), state.ConfigDir()} {
		if r, e := filepath.EvalSymlinks(managed); e == nil {
			managed = r
		}
		if root == managed || strings.HasPrefix(root, managed+string(filepath.Separator)) {
			return "", fmt.Errorf("choose a project-owned vault outside gtmux configuration/state")
		}
	}
	if create {
		if err := state.MkdirAll(root, 0700); err != nil {
			return "", err
		}
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("vault must be a directory")
	}
	return root, nil
}

// safeVaultFile rejects symlinks and traversal at every existing component.
func safeVaultFile(root, rel string) (string, error) {
	if filepath.IsAbs(rel) || filepath.Clean(rel) != rel || rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid vault path %q", rel)
	}
	p := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		p = filepath.Join(p, part)
		st, err := os.Lstat(p)
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing symlink in vault: %s", rel)
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return p, nil
}

func readVaultBase(root string) (vaultBase, error) {
	var b vaultBase
	p, err := safeVaultFile(root, vaultManifest)
	if err != nil {
		return b, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return b, err
	}
	if len(data) > 16<<20 {
		return b, fmt.Errorf("vault baseline exceeds 16 MiB")
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return b, err
	}
	if b.Version != 1 {
		return b, fmt.Errorf("unsupported vault version")
	}
	seen := map[string]bool{}
	for _, e := range b.Entries {
		if seen[e.ID] || e.ID == "" {
			return b, fmt.Errorf("duplicate/missing baseline id")
		}
		seen[e.ID] = true
		if _, err := safeVaultFile(root, e.Path); err != nil {
			return b, err
		}
	}
	return b, nil
}

func renderVaultEntry(e vaultEntry) string {
	return "---\ngtmux_id: " + strconv.Quote(e.ID) + "\ngtmux_revision: " + strconv.Quote(e.Revision) + "\ngtmux_topic: " + strconv.Quote(e.Topic) + "\n---\n\n# " + e.Title + "\n\n" + strings.TrimSpace(e.Body) + "\n"
}

// Only three generated scalar properties form the interchange header. A full
// YAML implementation is unnecessary: editable knowledge is Markdown prose;
// identity/scope/provenance are not silently imported from arbitrary YAML.
func parseVaultNote(data []byte) (vaultEntry, error) {
	var e vaultEntry
	if len(data) > 32<<10 {
		return e, fmt.Errorf("note exceeds 32 KiB")
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.HasPrefix(s, "---\n") {
		head, body, ok := strings.Cut(s[4:], "\n---\n")
		if !ok {
			return e, fmt.Errorf("unterminated properties")
		}
		props := map[string]string{}
		for _, line := range strings.Split(head, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			key, value, ok := strings.Cut(line, ":")
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			_, duplicate := props[key]
			if !ok || duplicate || !contains([]string{"gtmux_id", "gtmux_revision", "gtmux_topic"}, key) {
				return e, fmt.Errorf("only generated gtmux properties are supported; edit title/body or submit metadata in a review proposal")
			}
			if strings.HasPrefix(value, "\"") {
				v, err := strconv.Unquote(value)
				if err != nil {
					return e, err
				}
				value = v
			} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
			}
			props[key] = value
		}
		e.ID, e.Revision, e.Topic = props["gtmux_id"], props["gtmux_revision"], props["gtmux_topic"]
		s = body
	}
	s = strings.TrimSpace(s)
	title, body, _ := strings.Cut(s, "\n")
	if !strings.HasPrefix(title, "# ") {
		return e, fmt.Errorf("note needs a # title")
	}
	e.Title, e.Body = strings.TrimSpace(strings.TrimPrefix(title, "# ")), strings.TrimSpace(body)
	if e.Title == "" {
		return e, fmt.Errorf("empty title")
	}
	return e, nil
}

func scanVault(root string) (map[string]vaultEntry, []string, error) {
	notes := map[string]vaultEntry{}
	var drafts []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || contains([]string{"个人批注", "提案", "历史"}, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in vault: %s", rel)
		}
		if !strings.HasSuffix(d.Name(), ".md") || rel == "首页.md" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > 32<<10 {
			return fmt.Errorf("note %s exceeds 32 KiB", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		note, err := parseVaultNote(data)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		note.Path = rel
		if note.ID == "" {
			drafts = append(drafts, rel)
			return nil
		}
		if _, ok := notes[note.ID]; ok {
			return fmt.Errorf("duplicate id %s in vault; retain both files and resolve", note.ID)
		}
		notes[note.ID] = note
		return nil
	})
	return notes, drafts, err
}

func sameVaultText(a, b vaultEntry) bool {
	return a.Title == b.Title && strings.TrimSpace(a.Body) == strings.TrimSpace(b.Body)
}

// Publish each external note/baseline as a complete file. A stopped export must
// leave either revision readable, never a truncated baseline that loses edits.
// Callers first validate the vault path and compare the editor's text snapshot.
func publishVaultFile(path string, data []byte) error {
	if st, err := os.Lstat(path); err == nil {
		if !st.Mode().IsRegular() || st.Mode().Perm()&0222 == 0 {
			return fmt.Errorf("vault destination is not a writable regular file: %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := state.CreateTemp(filepath.Dir(path), ".knowledge-vault-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func ExportVault(path string) (VaultResult, error) {
	r := VaultResult{Conflicts: []string{}}
	root, err := vaultRoot(path, true)
	if err != nil {
		return r, err
	}
	base, err := readVaultBase(root)
	if os.IsNotExist(err) {
		entries, e := os.ReadDir(root)
		if e != nil {
			return r, e
		}
		for _, entry := range entries {
			if entry.Name() != ".obsidian" {
				return r, fmt.Errorf("choose an empty vault; refusing to adopt unrelated files")
			}
		}
		base = vaultBase{Version: 1}
		// Claim only the empty directory first; an interrupted export is retryable.
		if err := publishVaultFile(filepath.Join(root, vaultManifest), []byte("{\"version\":1,\"entries\":[]}\n")); err != nil {
			return r, err
		}
	} else if err != nil {
		return r, err
	}
	notes, _, err := scanVault(root)
	if err != nil {
		return r, err
	}
	previous := map[string]vaultEntry{}
	for _, e := range base.Entries {
		previous[e.ID] = e
	}
	live, err := liveKnowledge()
	if err != nil {
		return r, err
	}
	for _, e := range live {
		if e.Sensitive || e.Topic == "accounts" {
			r.Skipped++
			if _, exported := previous[e.ID]; exported {
				r.Conflicts = append(r.Conflicts, e.ID+": now excluded; existing local copy requires explicit removal, including editor backups")
			}
			continue
		}
		// The old topic/slug remains the stable identity; changing a title does
		// not force a filename migration or a new graph node.
		next := vaultEntry{ID: e.ID, Revision: entryRevision(e), Path: filepath.FromSlash(e.ID + ".md"), Topic: e.Topic, Title: e.Title, Body: e.Body}
		old, known := previous[e.ID]
		note, exists := notes[e.ID]
		if exists {
			// Recover a file written just before a prior export was interrupted.
			recovered := note.Revision == next.Revision && note.Topic == next.Topic && sameVaultText(note, next)
			if !recovered && (!known || note.Revision != old.Revision || note.Topic != old.Topic) {
				r.Conflicts = append(r.Conflicts, e.ID+": modified identity/baseline properties")
				continue
			}
			next.Path = note.Path
			if !recovered && !sameVaultText(note, old) && !sameVaultText(note, next) {
				r.Conflicts = append(r.Conflicts, e.ID+": preserved unapplied local edits")
				continue
			}
		}
		file, err := safeVaultFile(root, next.Path)
		if err != nil {
			return r, err
		}
		if !exists {
			if _, err := os.Lstat(file); err == nil {
				return r, fmt.Errorf("refusing to overwrite unrelated file %s", next.Path)
			} else if !os.IsNotExist(err) {
				return r, err
			}
		}
		if err := state.MkdirAll(filepath.Dir(file), 0700); err != nil {
			return r, err
		}
		if err := publishVaultFile(file, []byte(renderVaultEntry(next))); err != nil {
			return r, err
		}
		previous[e.ID] = next
		r.Written++
	}
	// Retired/excluded entries are retained in the baseline so local edits are
	// never silently discarded. diff reports their changed status explicitly.
	base.Entries = nil
	for _, e := range previous {
		base.Entries = append(base.Entries, e)
	}
	sort.Slice(base.Entries, func(i, j int) bool { return base.Entries[i].ID < base.Entries[j].ID })
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return r, err
	}
	manifest, err := safeVaultFile(root, vaultManifest)
	if err != nil {
		return r, err
	}
	if err := publishVaultFile(manifest, append(data, '\n')); err != nil {
		return r, err
	}
	index, err := safeVaultFile(root, "首页.md")
	if err != nil {
		return r, err
	}
	if _, err := os.Stat(index); os.IsNotExist(err) {
		text := "# gtmux 知识工作台\n\n编辑知识文件的标题与正文。生成的三个属性只用于版本校验。\n\n保存后运行 `gtmux knowledge review diff <vault>` 生成提案；在有权限的 HQ 入口预览并 apply。文件删除不等于知识退休。\n\n个人笔记放在 `个人批注/`，不会自动导入。无 ID 的新笔记只列为候选草稿，补齐依据后走 capture/add。\n\n刷新导出会保留未应用编辑；遇到版本冲突请比较后处理。此目录未自动启用云同步。\n"
		if err := publishVaultFile(index, []byte(text)); err != nil {
			return r, err
		}
	}
	return r, nil
}

func DiffVault(path string) (VaultResult, error) {
	r := VaultResult{Conflicts: []string{}}
	root, err := vaultRoot(path, false)
	if err != nil {
		return r, err
	}
	base, err := readVaultBase(root)
	if err != nil {
		return r, err
	}
	notes, drafts, err := scanVault(root)
	if err != nil {
		return r, err
	}
	r.Drafts = drafts
	live, err := liveKnowledge()
	if err != nil {
		return r, err
	}
	p := ReviewProposal{Version: 1, Origin: "external-editor", Changes: []ReviewChange{}}
	for _, old := range base.Entries {
		note, exists := notes[old.ID]
		if !exists {
			r.Missing = append(r.Missing, old.ID)
			continue
		}
		delete(notes, old.ID)
		if note.Revision != old.Revision || note.Topic != old.Topic {
			r.Conflicts = append(r.Conflicts, old.ID+": generated properties changed")
			continue
		}
		e, ok := findLive(live, old.ID)
		if !ok || e.Sensitive || e.Topic == "accounts" {
			r.Conflicts = append(r.Conflicts, old.ID+": retired or no longer eligible; local copy retained for explicit handling")
			continue
		}
		if sameVaultText(note, old) {
			continue
		}
		current := vaultEntry{Title: e.Title, Body: e.Body}
		if sameVaultText(note, current) {
			continue
		} // already applied; export refreshes the baseline
		if entryRevision(e) != old.Revision {
			r.Conflicts = append(r.Conflicts, old.ID+": central and local versions both changed")
			continue
		}
		p.Changes = append(p.Changes, ReviewChange{ID: old.ID, BaseRevision: old.Revision, Operation: "revise", Title: note.Title, Body: note.Body, Why: "reviewed external Markdown edit", Evidence: []string{"vault:" + note.Path}})
	}
	for id := range notes {
		r.Conflicts = append(r.Conflicts, id+": no trusted export baseline")
	}
	sort.Strings(r.Conflicts)
	sort.Strings(r.Missing)
	sort.Strings(r.Drafts)
	p.Conflicts, p.Missing, p.Drafts = r.Conflicts, r.Missing, r.Drafts
	p.ID = proposalDigest(p)
	r.Proposal = &p
	return r, nil
}
