package knowledge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
)

// MigrationLedgerFile is the knowledge-owned path inside an HQ archive.
const MigrationLedgerFile = "knowledge/.ledger.jsonl"

type MigrationEntry struct {
	ID        string   `json:"id"`
	Topic     string   `json:"topic"`
	Title     string   `json:"title"`
	Body      string   `json:"body,omitempty"`
	Sensitive bool     `json:"sensitive,omitempty"`
	Status    string   `json:"status"` // new | identical | conflict
	Records   int      `json:"records"`
	Lang      string   `json:"lang,omitempty"`
	Alt       *altHalf `json:"alt,omitempty"`
}
type MigrationResult struct {
	OperationID string `json:"operation_id"`
	Imported    int    `json:"imported"`
	Skipped     int    `json:"skipped"`
	Backup      string `json:"backup,omitempty"`
	Committed   bool   `json:"committed"`
}

func parseMigrationLedger(data []byte) ([]knowledgeOp, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("source ledger is not UTF-8")
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var out []knowledgeOp
	var topics []knowledgeOp
	liveIDs := map[string]bool{}
	allowed := map[string]bool{}
	for _, name := range []string{knowledgeOpAdd, knowledgeOpSupersede, knowledgeOpRetire, knowledgeOpTopic, knowledgeOpKind, knowledgeOpHit, knowledgeOpConfirm, knowledgeOpPromote, knowledgeOpLand, knowledgeOpWithdraw, knowledgeOpAlt, knowledgeOpSensitive, knowledgeOpDismiss} {
		allowed[name] = true
	}
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		if len(out) >= 50000 {
			return nil, errors.New("source ledger exceeds 50,000 records")
		}
		var op knowledgeOp
		decoder := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&op); err != nil {
			return nil, fmt.Errorf("invalid source ledger record %d", len(out)+1)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("invalid source ledger record %d", len(out)+1)
		}
		if op.ID == "" || !allowed[op.Op] || op.V > knowledgeSchemaV || op.V < 0 {
			return nil, fmt.Errorf("unsupported source ledger record %d", len(out)+1)
		}
		if err := validateKnowledgeContent(op.Title, op.Body, op.Why); err != nil {
			return nil, fmt.Errorf("source entry %s: %w", op.ID, err)
		}
		if err := validatePromotionFields(op.Target, op.Ref); err != nil {
			return nil, err
		}
		if err := validateAxes(op); err != nil {
			return nil, err
		}
		for _, s := range op.Sources {
			if s.ID == "" || s.Digest != "" && s.Digest != candidateDigest(s) {
				return nil, fmt.Errorf("invalid source snapshot in %s", op.ID)
			}
		}
		switch op.Op {
		case knowledgeOpTopic:
			if err := validateTopicName(op.ID, topics); err != nil {
				return nil, err
			}
			topics = append(topics, op)
		case knowledgeOpAdd, knowledgeOpSupersede:
			if !validKnowledgeTopic(op.Topic, topics) || !strings.HasPrefix(op.ID, op.Topic+"/") || len(op.ID) == len(op.Topic)+1 || strings.ContainsAny(op.ID, "\n\r\x00") {
				return nil, fmt.Errorf("invalid source entry ID/topic: %s", op.ID)
			}
			if op.Op == knowledgeOpAdd && liveIDs[op.ID] {
				return nil, fmt.Errorf("duplicate live source ID: %s", op.ID)
			}
			if op.Op == knowledgeOpSupersede {
				if !liveIDs[op.Supersedes] || op.ID != op.Supersedes && liveIDs[op.ID] {
					return nil, fmt.Errorf("invalid source supersede: %s", op.ID)
				}
				delete(liveIDs, op.Supersedes)
			}
			liveIDs[op.ID] = true
		case knowledgeOpRetire:
			delete(liveIDs, op.ID)
		}
		out = append(out, op)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// Supersede lineage must not loop or refer to absent content.
	content := map[string]bool{}
	parent := map[string]string{}
	for _, op := range out {
		if op.Op == knowledgeOpAdd || op.Op == knowledgeOpSupersede {
			content[op.ID] = true
		}
		if op.Op == knowledgeOpSupersede && op.ID != op.Supersedes {
			parent[op.ID] = op.Supersedes
		}
	}
	for id := range parent {
		seen := map[string]bool{}
		for cursor := id; cursor != ""; cursor = parent[cursor] {
			if seen[cursor] || !content[cursor] {
				return nil, fmt.Errorf("invalid source lineage for %s", id)
			}
			seen[cursor] = true
		}
	}
	return out, nil
}

func sourceFamily(ops []knowledgeOp, id string) ([]knowledgeOp, bool) {
	ids := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for _, op := range ops {
			if ids[op.ID] && op.Supersedes != "" && !ids[op.Supersedes] {
				ids[op.Supersedes] = true
				changed = true
			}
		}
	}
	var family []knowledgeOp
	sensitive := false
	for _, op := range ops {
		if ids[op.ID] && op.Op != knowledgeOpTopic {
			family = append(family, op)
			sensitive = sensitive || op.Sensitive
		}
	}
	return family, sensitive
}
func equivalentMigrationOp(a, b knowledgeOp) bool {
	a.Migration, b.Migration = "", ""
	return reflect.DeepEqual(a, b)
}
func migrationDestination(ops []knowledgeOp) map[string][]knowledgeOp {
	byID := map[string][]knowledgeOp{}
	for _, op := range ops {
		byID[op.ID] = append(byID[op.ID], op)
	}
	return byID
}
func migrationStatus(family []knowledgeOp, destination map[string][]knowledgeOp) string {
	overlap, identical := false, true
	for _, src := range family {
		found := false
		for _, dst := range destination[src.ID] {
			if dst.ID == src.ID && dst.Op != knowledgeOpTopic {
				overlap = true
			}
			if equivalentMigrationOp(src, dst) {
				found = true
			}
		}
		identical = identical && found
	}
	if identical {
		return "identical"
	}
	if overlap {
		return "conflict"
	}
	return "new"
}

// MigrationEntries exposes current source entries for an explicit local review.
// Sensitive families include any historical sensitive mark, even when later unmarked.
func MigrationEntries(data []byte, includeSensitive bool) ([]MigrationEntry, int, error) {
	dest, err := readKnowledgeOps()
	if err != nil {
		return nil, 0, err
	}
	return migrationEntries(data, includeSensitive, dest)
}

// MigrationSourceEntries validates source knowledge without reading a possibly damaged
// destination ledger. Complete-backup recovery must not depend on that ledger.
func MigrationSourceEntries(data []byte) ([]MigrationEntry, int, error) {
	return migrationEntries(data, false, nil)
}
func migrationEntries(data []byte, includeSensitive bool, dest []knowledgeOp) ([]MigrationEntry, int, error) {
	ops, err := parseMigrationLedger(data)
	if err != nil {
		return nil, 0, err
	}
	destinationIndex := migrationDestination(dest)
	entries := make([]MigrationEntry, 0)
	sensitiveCount := 0
	for _, entry := range foldKnowledge(ops) {
		family, sensitive := sourceFamily(ops, entry.ID)
		if sensitive {
			sensitiveCount++
			if !includeSensitive {
				continue
			}
		}
		entries = append(entries, MigrationEntry{ID: entry.ID, Topic: entry.Topic, Title: entry.Title, Body: entry.Body, Sensitive: sensitive, Status: migrationStatus(family, destinationIndex), Records: len(family), Lang: entry.Lang, Alt: entry.Alt})
	}
	return entries, sensitiveCount, nil
}

// MigrationSelection strips unrelated/retired families and all queue/control material.
func MigrationSelection(data []byte, includeSensitive bool) ([]byte, error) {
	ops, err := parseMigrationLedger(data)
	if err != nil {
		return nil, err
	}
	ids, topics := map[string]bool{}, map[string]bool{}
	for _, entry := range foldKnowledge(ops) {
		family, sensitive := sourceFamily(ops, entry.ID)
		if sensitive && !includeSensitive {
			continue
		}
		topics[entry.Topic] = true
		for _, op := range family {
			ids[op.ID] = true
		}
	}
	var out bytes.Buffer
	for _, op := range ops {
		if ids[op.ID] || op.Op == knowledgeOpTopic && topics[op.ID] {
			b, _ := json.Marshal(op)
			out.Write(b)
			out.WriteByte('\n')
		}
	}
	return out.Bytes(), nil
}

// ApplyMigration commits all selected, reviewed histories as one ledger replacement.
// Old audience decisions are withdrawn; no machine/repo instruction file is changed.
func ApplyMigration(data []byte, selected []string, source string) (result MigrationResult, err error) {
	result.OperationID = diag.NewOpID()
	if len(selected) == 0 {
		return result, errors.New("select at least one reviewed knowledge entry")
	}
	err = withKnowledgeLock(func() error {
		ops, err := parseMigrationLedger(data)
		if err != nil {
			return err
		}
		destination, err := readKnowledgeOps()
		if err != nil {
			return err
		}
		live := foldKnowledge(ops)
		destinationIndex := migrationDestination(destination)
		ids, topics := map[string]bool{}, map[string]bool{}
		imported := []string{}
		requested := map[string]bool{}
		for _, id := range selected {
			if requested[id] {
				continue
			}
			requested[id] = true
			entry, ok := findLive(live, id)
			if !ok {
				return fmt.Errorf("source entry %s is not live", id)
			}
			family, _ := sourceFamily(ops, id)
			switch migrationStatus(family, destinationIndex) {
			case "conflict":
				return fmt.Errorf("conflicting history for %s; current knowledge was not changed", id)
			case "identical":
				result.Skipped++
				continue
			}
			imported = append(imported, id)
			topics[entry.Topic] = true
			for _, op := range family {
				ids[op.ID] = true
			}
		}
		if len(imported) == 0 {
			return nil
		}
		var lines [][]byte
		custom := customTopics(destination)
		for _, op := range ops {
			if op.Op == knowledgeOpTopic && topics[op.ID] {
				if validKnowledgeTopic(op.ID, custom) {
					continue
				}
				if err := validateTopicName(op.ID, custom); err != nil {
					return err
				}
				custom = append(custom, op)
			} else if !ids[op.ID] {
				continue
			}
			op.Migration = source
			b, err := json.Marshal(op)
			if err != nil {
				return err
			}
			lines = append(lines, append(b, '\n'))
		}
		now := time.Now().Unix()
		for _, id := range imported {
			op := knowledgeOp{V: knowledgeSchemaV, Op: knowledgeOpWithdraw, ID: id, OpID: result.OperationID, At: now, Seq: events.LatestSeq(), Why: "Reviewed migration from another Mac; prior audience and promotion decisions are not carried", Migration: source}
			b, _ := json.Marshal(op)
			lines = append(lines, append(b, '\n'))
		}
		if _, err := os.Stat(knowledgeLedgerPath()); err == nil {
			result.Backup = knowledgeLedgerPath() + ".pre-migrate-" + result.OperationID
			if err := backupMigrationLedger(result.Backup); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		result.Committed, err = atomicAppendLines(knowledgeLedgerPath(), lines)
		if result.Committed {
			result.Imported = len(imported)
			events.AuditKnowledgeOutcome(fmt.Sprintf("migrate %s ×%d", source, result.Imported), now, result.OperationID, "committed", "ledger")
		}
		if err != nil {
			return err
		}
		active, custom, err := readKnowledgeState()
		if err != nil {
			return err
		}
		if err := renderAllTopics(active, custom, now); err != nil {
			return err
		}
		return renderPromotions(active)
	})
	if err != nil {
		phase := "ledger"
		outcome := "failed"
		if result.Committed {
			phase = "render-or-sync"
			outcome = "committed-view-failed"
		}
		events.AuditKnowledgeOutcome("migrate "+source, time.Now().Unix(), result.OperationID, outcome, phase)
		err = &CommitError{OpID: result.OperationID, Committed: result.Committed, Phase: phase, Err: err}
	}
	return result, err
}
func backupMigrationLedger(path string) error {
	src, err := os.Open(knowledgeLedgerPath())
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	if err == nil {
		err = dst.Sync()
	}
	closeErr := dst.Close()
	if err == nil {
		err = closeErr
	}
	return err
}
