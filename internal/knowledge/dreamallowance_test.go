package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDreamAllowancePreservesProgressCostsAndIdempotency(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			r := dreamFixture(t, 2, 1)
			ledger, _ := os.ReadFile(knowledgeLedgerPath())
			firstID := ""
			before, err := ResumeDream(context.Background(), r.ID, dreamChoice("first"), func(_ context.Context, call DreamCall) (DreamReply, error) {
				firstID = call.Item.ID
				if failed {
					return DreamReply{}, fmt.Errorf("synthetic failure")
				}
				return keepDream(call), nil
			})
			if err == nil || before.Calls != 1 {
				t.Fatal(before, err)
			}
			prefix, _ := os.ReadFile(dreamJournalPath())
			q := DreamAllowance{MaxCalls: 3, MaxSeconds: 200, Reason: "Owner explicitly added a bounded allowance"}
			after, err := ExtendDream(r.ID, q)
			if err != nil || after.Calls != before.Calls || after.Seconds != before.Seconds || after.UnknownUsage != before.UnknownUsage || len(after.Decisions) != len(before.Decisions) || after.Plan.ID != r.Plan.ID {
				t.Fatal(after, err)
			}
			data, _ := os.ReadFile(dreamJournalPath())
			if !bytes.HasPrefix(data, prefix) {
				t.Fatal("history rewritten")
			}
			if _, err = ExtendDream(r.ID, q); err != nil {
				t.Fatal(err)
			}
			repeated, _ := os.ReadFile(dreamJournalPath())
			if !bytes.Equal(data, repeated) {
				t.Fatal("identical extension not idempotent")
			}
			calls := 0
			after, err = ResumeDream(context.Background(), r.ID, dreamChoice("second"), func(_ context.Context, call DreamCall) (DreamReply, error) {
				calls++
				if !failed && call.Item.ID == firstID {
					t.Error("completed item replayed")
				}
				return keepDream(call), nil
			})
			want := 1
			if failed {
				want = 2
			}
			if err != nil || after.State != "review_ready" || calls != want || after.Calls != 1+want || after.UnknownUsage != before.UnknownUsage {
				t.Fatal(after, err, calls)
			}
			if _, err = ExtendDream(r.ID, DreamAllowance{MaxCalls: 4, MaxSeconds: 200, Reason: "nothing pending"}); err == nil {
				t.Fatal("expanded completed work")
			}
			end, _ := os.ReadFile(knowledgeLedgerPath())
			if !bytes.Equal(end, ledger) {
				t.Fatal("KB changed")
			}
		})
	}
}

func TestDreamAllowanceRejectsInvalidActiveAndRetryBypass(t *testing.T) {
	r := dreamFixture(t, 1, 1)
	prefix, _ := os.ReadFile(dreamJournalPath())
	for _, q := range []DreamAllowance{{MaxCalls: 2, MaxSeconds: 100}, {MaxCalls: 61, MaxSeconds: 100, Reason: "x"}, {MaxCalls: 2, MaxSeconds: 14401, Reason: "x"}, {MaxCalls: 2, MaxSeconds: 99, Reason: "x"}} {
		if _, err := ExtendDream(r.ID, q); err == nil {
			t.Fatal("invalid extension accepted", q)
		}
	}
	after, _ := os.ReadFile(dreamJournalPath())
	if !bytes.Equal(prefix, after) {
		t.Fatal("invalid request wrote history")
	}
	q := DreamAllowance{MaxCalls: 4, MaxSeconds: 200, Reason: "Authorized retry budget"}
	request := filepath.Join(t.TempDir(), "allowance.json")
	data, _ := json.Marshal(q)
	if err := os.WriteFile(request, data, 0600); err != nil {
		t.Fatal(err)
	}
	if text := captureStdout(t, func() {
		if code := knowledgeDream([]string{"extend", r.ID, request}); code != 0 {
			t.Error("HQ fixture CLI refused", code)
		}
	}); text == "" {
		t.Fatal("no CLI receipt")
	}
	t.Chdir(t.TempDir())
	if code := knowledgeDream([]string{"extend", r.ID, request}); code == 0 {
		t.Fatal("non-HQ writer accepted")
	}
	for i := 0; i < 3; i++ {
		_, _ = ResumeDream(context.Background(), r.ID, dreamChoice("fake"), func(context.Context, DreamCall) (DreamReply, error) { return DreamReply{}, fmt.Errorf("failure") })
	}
	if _, err := ExtendDream(r.ID, DreamAllowance{MaxCalls: 5, MaxSeconds: 200, Reason: "Cannot remove retry guard"}); err != nil {
		t.Fatal(err)
	}
	called := false
	status, err := ResumeDream(context.Background(), r.ID, dreamChoice("fake"), func(context.Context, DreamCall) (DreamReply, error) { called = true; return DreamReply{}, nil })
	if err == nil || called || status.Calls != 3 {
		t.Fatal("extension bypassed retry bound", status, err)
	}

	t.Run("active", func(t *testing.T) {
		r := dreamFixture(t, 1, 1)
		func() {
			defer func() {
				if recover() == nil {
					t.Error("missing interruption")
				}
			}()
			_, _ = ResumeDream(context.Background(), r.ID, dreamChoice("fake"), func(context.Context, DreamCall) (DreamReply, error) { panic("controller interrupted") })
		}()
		if _, err := ExtendDream(r.ID, q); err == nil {
			t.Fatal("unresolved attempt extended")
		}
	})
}
