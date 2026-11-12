package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestDreamDiagnosticsJournalAllowlistAndLegacyEncoding(t *testing.T) {
	b, _ := json.Marshal(DreamReply{})
	if string(b) != `{"usage_known":false}` {
		t.Fatal("legacy reply encoding changed", string(b))
	}
	r := dreamFixture(t, 1, 3)
	code := 2
	const secret = "private-session-url-or-credential"
	status, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(context.Context, DreamCall) (DreamReply, error) {
		return DreamReply{FailureCode: secret, ExitCode: &code}, fmt.Errorf("%s", secret)
	})
	if err == nil || status.Calls != 1 || status.UnknownUsage != 1 || status.Seconds < 1 || !strings.Contains(status.Reason, "executor (exit 2)") {
		t.Fatal(status, err)
	}
	data, _ := os.ReadFile(dreamJournalPath())
	if strings.Contains(string(data), secret) {
		t.Fatal("untrusted diagnostic archived")
	}
	history, err := DreamHistory(r.ID)
	if err != nil || history[len(history)-1].Reply.FailureCode != "executor" {
		t.Fatal(history, err)
	}
	status, err = ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(_ context.Context, call DreamCall) (DreamReply, error) {
		reply := keepDream(call)
		reply.FailureCode, reply.ExitCode = secret, &code
		return reply, nil
	})
	if err != nil || status.Calls != 2 || status.UnknownUsage != 1 || status.State != "review_ready" {
		t.Fatal(status, err)
	}
	history, err = DreamHistory(r.ID)
	if err != nil || history[len(history)-1].Reply.FailureCode != "" || history[len(history)-1].Reply.ExitCode != nil {
		t.Fatal(history, err)
	}
}
