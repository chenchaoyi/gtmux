package knowledge

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Distribution (hq-knowledge-engine D5–D6): a lesson filed for an audience reaches that
// audience's readers, or it was never learned by anyone but HQ.
//
//   - hq       → appended to the supervisor's own LOCAL.md, once per entry.
//   - machine  → ONE canonical file (MachinePath), plus a managed block in each supported
//                agent's global instruction file carrying an INDEX: one line per entry and
//                the canonical path. Never the full text — the block sits in every session's
//                context, and an index cannot grow into a problem (Skills' progressive
//                disclosure). Codex, opencode and Kimi cannot import a file; all four can
//                read one when the index points at it.
//   - repo     → the same managed block, full text, in that repository's AGENTS.md (or
//                CLAUDE.md when only that exists). gtmux never commits.
//   - everyone → a GitHub issue prefilled from the brief; landed with the issue URL.
//
// Every write to a file gtmux does not own touches ONLY the inside of its own sentinel
// block; a block whose content no longer matches the hash gtmux stamped is hand-edited,
// and sync refuses to overwrite it without --force.

// Carrier is one agent's global instruction file.
type Carrier struct {
	Agent string `json:"agent"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

// Carriers lists the supported agents' global instruction files, resolved on this
// machine. An agent whose manifest names no instruction file is not a carrier.
func Carriers() []Carrier {
	var out []Carrier
	for _, m := range agents.All() {
		if m.Instructions == "" {
			continue
		}
		out = append(out, Carrier{Agent: m.Key, Label: m.Label, Path: resolveInstructions(m)})
	}
	return out
}

// resolveInstructions expands the manifest's path: `~` is the user's home, and when the
// agent's home env var is set the file moves with it (the same rule the transcript
// readers apply to $CODEX_HOME / $KIMI_CODE_HOME).
func resolveInstructions(m agents.Manifest) string {
	if m.InstructionsEnv != "" {
		if h := os.Getenv(m.InstructionsEnv); h != "" {
			return filepath.Join(h, filepath.Base(m.Instructions))
		}
	}
	p := m.Instructions
	if strings.HasPrefix(p, "~/") {
		p = filepath.Join(state.Home(), p[2:])
	}
	return p
}

const (
	blockBeginPrefix = "<!-- gtmux:knowledge:begin v1 "
	blockBeginSuffix = " -->"
	blockEnd         = "<!-- gtmux:knowledge:end -->"
)

func bodyHash(body string) string {
	h := sha1.Sum([]byte(body))
	return hex.EncodeToString(h[:])[:12]
}

// wrapBlock puts the sentinels around a body, stamping the body's hash so a later read
// can tell "gtmux wrote this and it is stale" from "someone edited inside".
func wrapBlock(body string) string {
	return blockBeginPrefix + bodyHash(body) + blockBeginSuffix + "\n" + body + "\n" + blockEnd + "\n"
}

// findBlock locates gtmux's block in a file. start..end spans the whole block including
// sentinels and the trailing newline; hash is the stamped one; body the text between.
func findBlock(content string) (start, end int, hash, body string, ok bool) {
	i := strings.Index(content, blockBeginPrefix)
	if i < 0 {
		return 0, 0, "", "", false
	}
	lineEnd := strings.Index(content[i:], "\n")
	if lineEnd < 0 {
		return 0, 0, "", "", false
	}
	head := content[i : i+lineEnd]
	hash = strings.TrimSuffix(strings.TrimPrefix(head, blockBeginPrefix), blockBeginSuffix)
	j := strings.Index(content[i:], blockEnd)
	if j < 0 {
		return 0, 0, "", "", false
	}
	bodyStart := i + lineEnd + 1
	bodyEnd := i + j
	body = strings.TrimSuffix(content[bodyStart:bodyEnd], "\n")
	end = i + j + len(blockEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return i, end, hash, body, true
}

// SyncState is one carrier's relation to the ledger.
type SyncState string

const (
	SyncInSync       SyncState = "in-sync"
	SyncMissingFile  SyncState = "missing-file"
	SyncMissingBlock SyncState = "missing-block"
	SyncStale        SyncState = "stale"       // gtmux wrote it; the ledger moved on
	SyncHandEdited   SyncState = "hand-edited" // the block's text no longer matches its stamp
)

// CarrierStatus is a carrier with its state.
type CarrierStatus struct {
	Carrier
	State SyncState `json:"state"`
}

func stateOf(content, want string) SyncState {
	_, _, hash, body, ok := findBlock(content)
	switch {
	case !ok:
		return SyncMissingBlock
	case body == want:
		return SyncInSync
	case bodyHash(body) == hash:
		return SyncStale
	default:
		return SyncHandEdited
	}
}

// machineIndex is the block body for the machine audience: an index, never the text.
func machineIndex(live []knowledgeOp) string {
	var b strings.Builder
	b.WriteString("gtmux · knowledge every agent on this machine must know · 本机所有 agent 都该知道的\n")
	b.WriteString("Full text, with the reasons and examples: " + MachinePath() + "\n")
	n := 0
	for _, op := range live {
		if op.Audience != AudienceMachine || op.Status == StatusHypothesis {
			continue
		}
		b.WriteString("- [" + op.Kind + "] " + op.Title)
		if head := firstLine(op.Body); head != "" {
			b.WriteString(" — " + clip(head, 120))
		}
		b.WriteString("\n")
		n++
	}
	if n == 0 {
		b.WriteString("- (nothing distributed yet)\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func firstLine(s string) string {
	for _, ln := range strings.Split(strings.TrimSpace(s), "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// CarrierStatuses reads the ledger and reports every carrier's state. No ledger reads
// as "everything in sync with nothing" — a machine with no HQ has nothing to distribute.
func CarrierStatuses() ([]CarrierStatus, error) {
	live, err := liveKnowledge()
	if err != nil {
		return nil, err
	}
	return carrierStatuses(live), nil
}

func carrierStatuses(live []knowledgeOp) []CarrierStatus {
	want := machineIndex(live)
	var out []CarrierStatus
	for _, c := range Carriers() {
		b, err := os.ReadFile(c.Path)
		st := CarrierStatus{Carrier: c}
		if err != nil {
			st.State = SyncMissingFile
		} else {
			st.State = stateOf(string(b), want)
		}
		out = append(out, st)
	}
	return out
}

// SyncReport is what one sync did.
type SyncReport struct {
	Written []string `json:"written"` // carriers whose block was created or refreshed
	Kept    []string `json:"kept"`    // already in sync
	Refused []string `json:"refused"` // hand-edited and no --force
}

// SyncMachine refreshes the machine block in every carrier. A missing file is created
// holding only the block; a hand-edited block is refused unless force.
func SyncMachine(force bool) (SyncReport, error) {
	live, custom, err := readKnowledgeState()
	if err != nil {
		return SyncReport{}, err
	}
	// The canonical file first: the index points at it.
	if err := writeMachineRender(live); err != nil {
		return SyncReport{}, err
	}
	_ = custom
	return syncBlocks(live, force)
}

func syncBlocks(live []knowledgeOp, force bool) (SyncReport, error) {
	var rep SyncReport
	want := machineIndex(live)
	for _, c := range Carriers() {
		changed, refused, err := installBlock(c.Path, want, force)
		if err != nil {
			return rep, fmt.Errorf("%s (%s): %w", c.Label, c.Path, err)
		}
		switch {
		case refused:
			rep.Refused = append(rep.Refused, c.Agent)
		case changed:
			rep.Written = append(rep.Written, c.Agent)
		default:
			rep.Kept = append(rep.Kept, c.Agent)
		}
	}
	return rep, nil
}

// installBlock writes the block into path, touching nothing outside it. Returns whether
// it wrote, or refused a hand-edited block.
func installBlock(path, body string, force bool) (changed, refused bool, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, false, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return false, false, err
		}
		return true, false, os.WriteFile(path, []byte(wrapBlock(body)), 0o644)
	}
	content := string(b)
	start, end, _, _, ok := findBlock(content)
	if !ok {
		sep := ""
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			sep = "\n"
		}
		if len(content) > 0 {
			sep += "\n"
		}
		return true, false, os.WriteFile(path, []byte(content+sep+wrapBlock(body)), 0o644)
	}
	switch stateOf(content, body) {
	case SyncInSync:
		return false, false, nil
	case SyncHandEdited:
		if !force {
			return false, true, nil
		}
	}
	return true, false, os.WriteFile(path, []byte(content[:start]+wrapBlock(body)+content[end:]), 0o644)
}

// RepoCarrierPath is the instruction file gtmux writes a repo block into: AGENTS.md when
// it exists, CLAUDE.md when only that does, otherwise AGENTS.md (created).
func RepoCarrierPath(repo string) string {
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(repo, name)); err == nil {
			return filepath.Join(repo, name)
		}
	}
	return filepath.Join(repo, "AGENTS.md")
}

// repoBlock is the block body for one repository: full text, because it is small and
// only the agents working there read it.
func repoBlock(live []knowledgeOp, repo string) string {
	var b strings.Builder
	b.WriteString("gtmux · knowledge for agents working in this repository · 在这个仓库干活的 agent 该知道的\n")
	n := 0
	for _, op := range live {
		if op.Audience != AudienceRepo || op.AudienceRepo != repo || op.Status == StatusHypothesis {
			continue
		}
		b.WriteString("\n### " + op.Title + "\n")
		if body := strings.TrimSpace(op.Body); body != "" {
			b.WriteString(body + "\n")
		}
		b.WriteString("_" + op.ID + " · " + stampDate(op.At) + "_\n")
		n++
	}
	if n == 0 {
		b.WriteString("- (nothing yet)\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// SyncRepo writes the repo block into that repository's instruction file. It never
// commits; the caller says so to the person.
func SyncRepo(repo string, force bool) (path string, refused bool, err error) {
	live, err := liveKnowledge()
	if err != nil {
		return "", false, err
	}
	repo = cleanRepo(repo)
	path = RepoCarrierPath(repo)
	_, refused, err = installBlock(path, repoBlock(live, repo), force)
	return path, refused, err
}

func cleanRepo(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(p)
}

// LocalPath is the supervisor's own LOCAL.md — the hq audience's carrier.
func LocalPath() string { return filepath.Join(state.HQHome(), "LOCAL.md") }

// carryIntoLocal appends one entry to LOCAL.md as its own section, once: the marker
// carrying the id makes a second land a no-op instead of a duplicate. LOCAL.md is the
// operator's file; nothing already in it is touched.
func carryIntoLocal(op knowledgeOp) (string, error) {
	path := LocalPath()
	marker := "<!-- gtmux:knowledge " + op.ID + " -->"
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return path, err
	}
	content := string(b)
	if strings.Contains(content, marker) {
		return path, nil
	}
	var s strings.Builder
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		s.WriteString("\n")
	}
	s.WriteString("\n## " + op.Title + "\n\n")
	if body := strings.TrimSpace(op.Body); body != "" {
		s.WriteString(body + "\n\n")
	}
	s.WriteString(marker + "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, []byte(content+s.String()), 0o644)
}

// IssueURL is the everyone audience's exit for a person who never opens a pull request:
// a new-issue page on the product's tracker, prefilled from the brief. The body is
// bounded so the URL stays within what browsers accept.
func IssueURL(op knowledgeOp) string {
	const repo = "https://github.com/chenchaoyi/gtmux/issues/new"
	title := "[knowledge] " + op.Title
	body := renderPromotionCore(op)
	if len(body) > 6000 {
		body = body[:6000] + "\n…"
	}
	return repo + "?title=" + url.QueryEscape(title) + "&body=" + url.QueryEscape(body)
}
