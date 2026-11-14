package codexbridge

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

// Positive controller control: producer assertions are in resume's CFL006. This
// bind seam exercises the real observer/recovery state machine, not real IO/E2E.
func TestCFL006InitialRootMetadataRetryCommitsThenResumesHeartbeats(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	b.attachment.Loc = "work:0.0"
	attempts := 0
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		attempts++
		*calls = append(*calls, "metadata:"+id)
		if attempts == 1 {
			return f, &resume.FrontendBindingError{Code: "frontend-publication-unavailable", Retryable: true}
		}
		f.SessionID, f.Loc = id, "work:2.0"
		return f, nil
	}
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	o.response(rootResponse(1, "observed-root"))
	if b.pending == nil || !b.suspended || b.attachment.SessionID != "" {
		t.Fatal("partial initial root incorrectly committed", b.attachment, b.pending, b.suspended)
	}
	*clock = clock.Add(time.Second)
	b.tick()
	if b.pending != nil || b.suspended || b.attachment.SessionID != "observed-root" || b.attachment.Loc != "work:2.0" {
		t.Fatal("metadata retry left first-root controller suspended", b.attachment, b.pending, b.suspended)
	}
	for i := 0; i < 5; i++ {
		*clock = clock.Add(time.Second)
		b.tick()
	}
	if attempts != 2 || len(*calls) != 7 {
		t.Fatal("later ticks reissued metadata/root or omitted normal heartbeats", attempts, *calls)
	}
	b.close()
	b.tick()
	if attempts != 2 || len(*calls) != 7 {
		t.Fatal("retired controller resumed", attempts, *calls)
	}
}
