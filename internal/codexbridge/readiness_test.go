package codexbridge

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestCFR003AdmissionRetriesOnlyTransientEvidence(t *testing.T) {
	for _, kind := range []string{"transient-ready", "hard", "exhausted", "cancel-after-check", "cancel-in-pause", "late-success"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, pauses := 0, 0
			f, n, err := awaitFrontend(ctx, func(received context.Context) (resume.CodexFrontend, error) {
				if received != ctx {
					t.Fatal("attempt obtained a fresh budget")
				}
				calls++
				if kind == "cancel-after-check" || kind == "late-success" {
					cancel()
				}
				if kind == "late-success" || kind == "transient-ready" && calls == 2 {
					return resume.CodexFrontend{Instance: "verified"}, nil
				}
				return resume.CodexFrontend{}, &resume.FrontendAdmissionError{Code: map[bool]string{true: "peer-mismatch", false: "stdin-unavailable"}[kind == "hard"], Retryable: kind != "hard"}
			}, func(received context.Context) error {
				pauses++
				if received != ctx {
					t.Fatal("pause obtained fresh budget")
				}
				if kind == "cancel-in-pause" {
					cancel()
					return ctx.Err()
				}
				return nil
			})
			want := map[string]int{"transient-ready": 2, "hard": 1, "exhausted": 4, "cancel-after-check": 1, "cancel-in-pause": 1, "late-success": 1}[kind]
			if calls != want || n != want {
				t.Fatalf("calls=%d attempts=%d want=%d", calls, n, want)
			}
			if kind == "transient-ready" {
				if err != nil || f.Instance != "verified" || pauses != 1 {
					t.Fatalf("transient gap did not recover: %+v %v", f, err)
				}
			} else if err == nil || f.Instance != "" {
				t.Fatalf("unsafe acceptance: %+v %v", f, err)
			}
			if kind == "hard" && (pauses != 0 || admissionReason(err) != "peer-mismatch") {
				t.Fatal("hard mismatch was retried", pauses, err)
			}
			if kind == "hard" && admissionStatus(err) != http.StatusForbidden {
				t.Fatal("hard refusal lost its status", err)
			}
			if kind == "exhausted" && admissionStatus(err) != http.StatusServiceUnavailable {
				t.Fatal("temporary missing evidence reported as permission denial", err)
			}
			if kind == "exhausted" && pauses != 3 {
				t.Fatal("unbounded retries", pauses)
			}
		})
	}
}

func TestCFR003LaunchPublicationAndCancellation(t *testing.T) {
	g := newLaunchGate()
	entered := make(chan struct{})
	done := make(chan int)
	go func() { close(entered); pid, _ := g.wait(context.Background()); done <- pid }()
	<-entered
	select {
	case <-done:
		t.Fatal("read unpublished child")
	default:
	}
	g.publish(300)
	if pid := <-done; pid != 300 {
		t.Fatal("wrong published child", pid)
	}
	g.publish(999)
	if pid, err := g.wait(context.Background()); err != nil || pid != 300 {
		t.Fatal("launch identity replaced", pid, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newLaunchGate().wait(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal("gate ignored cancellation", err)
	}
	failed := newLaunchGate()
	failed.publish(0)
	if _, err := failed.wait(context.Background()); admissionReason(err) != "launch-unavailable" {
		t.Fatal("launch failure lost", err)
	}
	if err := admissionPause(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal("pause ignored cancellation", err)
	}
}
