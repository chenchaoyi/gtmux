package knowledge

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UsageReceipt is an explicit report, not proof of adoption or causality. It
// contains no task prose or transcript. Retrieval itself remains read-only.
type UsageReceipt struct {
	Version      int         `json:"version"`
	ID           string      `json:"id"`
	At           int64       `json:"at"`
	TaskKey      string      `json:"task_key"`
	PlanID       string      `json:"plan_id"`
	Purpose      string      `json:"purpose"`
	Stage        string      `json:"stage"`
	Observation  string      `json:"observation"`
	Items        []UsageItem `json:"items"`
	Outcome      string      `json:"outcome,omitempty"`
	EvidenceHash string      `json:"evidence_hash,omitempty"`
}
type UsageItem struct {
	ID          string `json:"id"`
	Revision    string `json:"revision"`
	ContentHash string `json:"content_hash"`
}
type UsageSummary struct {
	Reports           int            `json:"reports"`
	Tasks             int            `json:"tasks"`
	Stages            map[string]int `json:"self_reported_stages"`
	EvaluationReports int            `json:"evaluation_reports"`
	CausalUseRate     *float64       `json:"causal_use_rate"`
	Limitation        string         `json:"limitation"`
}

func usagePath() string { return filepath.Join(Dir(), ".memory-usage.jsonl") }
func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
func usageDigest(r UsageReceipt) string {
	r.ID = ""
	r.At = 0
	b, _ := json.Marshal(r)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func validateUsage(r UsageReceipt) error {
	if r.Version != 1 || !validDigest(r.TaskKey) || !validDigest(r.PlanID) {
		return fmt.Errorf("usage needs version 1 and SHA-256 task_key/plan_id; do not submit task prose")
	}
	if !contains([]string{"task", "evaluation"}, r.Purpose) || r.Observation != "self_report" {
		return fmt.Errorf("usage accepts task/evaluation self_report only; audit is excluded")
	}
	if !contains([]string{"retrieved", "shown", "adopted", "outcome"}, r.Stage) || len(r.Items) == 0 || len(r.Items) > 6 {
		return fmt.Errorf("usage needs a known stage and 1–6 item revisions")
	}
	seen := map[string]bool{}
	for _, item := range r.Items {
		if item.ID == "" || len(item.ID) > 300 || seen[item.ID] || !validDigest(item.Revision) || !validDigest(item.ContentHash) {
			return fmt.Errorf("invalid/duplicate usage item")
		}
		seen[item.ID] = true
	}
	if r.Stage == "outcome" {
		if !contains([]string{"succeeded", "failed", "unknown"}, r.Outcome) {
			return fmt.Errorf("outcome must be succeeded, failed, or unknown")
		}
	} else if r.Outcome != "" {
		return fmt.Errorf("outcome belongs only to the outcome stage")
	}
	if r.EvidenceHash != "" && !validDigest(r.EvidenceHash) {
		return fmt.Errorf("evidence_hash must be SHA-256, never a private URL")
	}
	if r.ID != "" && r.ID != usageDigest(r) {
		return fmt.Errorf("usage digest mismatch")
	}
	return nil
}

func readUsage() ([]UsageReceipt, error) {
	f, err := os.Open(usagePath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 8<<20 {
		return nil, fmt.Errorf("usage file exceeds 8 MiB; archive explicitly before recording more")
	}
	var rows []UsageReceipt
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 32<<10)
	for s.Scan() {
		var r UsageReceipt
		if err := json.Unmarshal(s.Bytes(), &r); err != nil {
			return nil, err
		}
		if err := validateUsage(r); err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, s.Err()
}

func RecordUsage(r UsageReceipt) (UsageReceipt, error) {
	if err := validateUsage(r); err != nil {
		return r, err
	}
	r.ID = usageDigest(r)
	r.At = time.Now().Unix()
	err := withKnowledgeLock(func() error {
		rows, err := readUsage()
		if err != nil {
			return err
		}
		for _, old := range rows {
			if old.ID == r.ID {
				r = old
				return nil
			}
		}
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		var size int64
		if st, err := os.Stat(usagePath()); err == nil {
			size = st.Size()
		} else if !os.IsNotExist(err) {
			return err
		}
		if size+int64(len(b))+1 > 8<<20 {
			return fmt.Errorf("usage storage cap reached; archive explicitly")
		}
		_, err = atomicAppendLines(usagePath(), [][]byte{append(b, '\n')})
		return err
	})
	return r, err
}

func SummarizeUsage(since int64) (UsageSummary, error) {
	r := UsageSummary{Stages: map[string]int{}, Limitation: "Self-reported stages, not verified causal reuse. Missing records do not mean unused; evaluation is separate. Manual hit remains recurrence."}
	rows, err := readUsage()
	if err != nil {
		return r, err
	}
	tasks := map[string]bool{}
	for _, row := range rows {
		if row.At < since {
			continue
		}
		if row.Purpose == "evaluation" {
			r.EvaluationReports++
			continue
		}
		r.Reports++
		r.Stages[row.Stage]++
		tasks[row.TaskKey] = true
	}
	r.Tasks = len(tasks)
	return r, nil
}
