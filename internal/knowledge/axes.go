package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// The three axes (hq-knowledge-engine D2–D4). Every live entry answers three questions
// a reader — HQ or a person — would otherwise have to guess at: WHAT it is (kind), WHERE
// it came from and how often (provenance), and WHO must know it (audience). They are
// orthogonal: a pitfall learned from a correction may be for everyone; a fact captured
// by a worker may be for this machine only. `topic` survives as a free tag — the ids
// and the rendered topic files are keyed on it — so the vocabulary HQ has declared keeps
// working unchanged.

// Kinds, CoALA's semantic/procedural split one level finer: facts (how the world is),
// howto / pitfalls / judgment (how to act: do this, never that, decide by this), and
// decisions (why this was chosen over that — the record PRs and proposals used to hold).
const (
	KindFacts     = "facts"
	KindHowto     = "howto"
	KindPitfalls  = "pitfalls"
	KindJudgment  = "judgment"
	KindDecisions = "decisions"
)

// Kinds is the vocabulary, in the order surfaces list it.
var Kinds = []string{KindFacts, KindHowto, KindPitfalls, KindJudgment, KindDecisions}

// Provenance kinds: how the lesson reached the ledger. `correction` is the commander
// pushing back; `recurrence` a footgun hit again; `mined` the transcript miner; `capture`
// a worker's `gtmux capture`; `self` HQ's own observation.
const (
	ProvCorrection = "correction"
	ProvRecurrence = "recurrence"
	ProvMined      = "mined"
	ProvCapture    = "capture"
	ProvSelf       = "self"
)

var Provenances = []string{ProvCorrection, ProvRecurrence, ProvMined, ProvCapture, ProvSelf}

// Audiences: who must know. The four words the UI shows are these.
const (
	AudienceHQ       = "hq"       // 中控 — LOCAL.md
	AudienceMachine  = "machine"  // 本机 — every agent on this machine
	AudienceRepo     = "repo"     // 仓库 — the agents working in one repository
	AudienceEveryone = "everyone" // 全体 — the product
)

var Audiences = []string{AudienceHQ, AudienceMachine, AudienceRepo, AudienceEveryone}

// StatusHypothesis marks a filed lead that is not yet verified: it renders in its own
// section and is never distributed. The zero status is "live".
const StatusHypothesis = "hypothesis"

// legacyKind is the read-time migration table for records written before the kind
// axis existed (D10). Built-in topics map deterministically; a declared topic is
// almost always a body of facts about a domain. Every mapped kind is ASSUMED — lint
// asks HQ to confirm — because the record itself never said.
var legacyKind = map[string]string{
	"environment":    KindFacts,
	"accounts":       KindFacts,
	"workflows":      KindHowto,
	"pitfalls":       KindPitfalls,
	"best-practices": KindJudgment,
	"corrections":    KindJudgment,
}

func kindForTopic(topic string) string {
	if k, ok := legacyKind[topic]; ok {
		return k
	}
	return KindFacts
}

// applyMigration fills the axes on a record that predates them. It runs at fold time,
// on the in-memory entry; the file is never rewritten.
func applyMigration(op *knowledgeOp) {
	if op.Kind == "" {
		op.Kind = kindForTopic(op.Topic)
		op.KindAssumed = true
	}
	if op.Provenance == "" {
		switch {
		case op.Topic == "corrections":
			op.Provenance = ProvCorrection
		case op.Capture != "":
			op.Provenance = ProvCapture
		default:
			op.Provenance = ProvSelf
		}
	}
	if op.Hits == 0 {
		op.Hits = 1
	}
}

func validKind(k string) bool       { return contains(Kinds, k) }
func validProvenance(p string) bool { return contains(Provenances, p) }
func validAudience(a string) bool   { return contains(Audiences, a) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// validateAxes refuses an op that names a value outside the vocabularies. Empty is
// allowed on the op (the fold fills it); a wrong word is not.
func validateAxes(op knowledgeOp) error {
	if op.Kind != "" && !validKind(op.Kind) {
		return fmt.Errorf("unknown kind %q (want %s)", op.Kind, strings.Join(Kinds, " | "))
	}
	if op.Provenance != "" && !validProvenance(op.Provenance) {
		return fmt.Errorf("unknown provenance %q (want %s)", op.Provenance, strings.Join(Provenances, " | "))
	}
	if op.Audience != "" && !validAudience(op.Audience) {
		return fmt.Errorf("unknown audience %q (want %s)", op.Audience, strings.Join(Audiences, " | "))
	}
	if op.Status != "" && op.Status != StatusHypothesis {
		return fmt.Errorf("unknown status %q", op.Status)
	}
	return nil
}

// ledgerBackupOnce copies the v1 ledger aside before the first v2 record lands, so the
// migration is reversible by hand. It runs once: the marker is the backup's existence.
func ledgerBackupOnce() error {
	src := knowledgeLedgerPath()
	b, err := os.ReadFile(src)
	if err != nil {
		return nil // no ledger yet — nothing to back up
	}
	if strings.Contains(string(b), `"v":2`) {
		return nil // already migrated
	}
	dst := src + ".bak-v1"
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	return os.WriteFile(dst, b, 0o644)
}

// MachinePath is the canonical file for audience `machine`: what every agent on this
// machine must know, rendered once by gtmux and fanned out as an index (phase 3). It
// lives beside the user's gtmux config, outside the HQ home, because it belongs to the
// machine, not to the supervisor.
func MachinePath() string {
	return filepath.Join(filepath.Dir(usercfg.Path()), "knowledge", "machine.md")
}

// HitByCaptureKey records that a lesson already filed was hit again: the entry whose
// consumed capture keys include key gets a `hit` op of n. It is the miner's feedback
// path (a recurring-error signature the ledger already holds keeps counting), so it
// runs without the HQ-home gate — the caller is gtmux itself, journaled as such.
func HitByCaptureKey(key string, n int, now int64) (id string, ok bool, err error) {
	if n <= 0 {
		return "", false, nil
	}
	live, err := liveKnowledge()
	if err != nil {
		return "", false, err
	}
	for _, op := range live {
		if !contains(splitKeys(op.Capture), key) {
			continue
		}
		hit := knowledgeOp{Op: knowledgeOpHit, ID: op.ID, Hits: n, At: now, Seq: events.LatestSeq(), Why: "miner: " + key}
		if err := appendKnowledgeOp(hit); err != nil {
			return op.ID, false, err
		}
		events.AuditKnowledge(fmt.Sprintf("hit %s ×%d (miner %s)", op.ID, n, key), now)
		return op.ID, true, nil
	}
	return "", false, nil
}
