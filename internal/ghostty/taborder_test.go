package ghostty

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTabOrderBulkAndLegacyShareDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var scripts []string
	got, err := readTabOrder(ctx, func(received context.Context, script string) (string, error) {
		if received != ctx {
			t.Fatal("fallback reset the read budget")
		}
		scripts = append(scripts, script)
		if len(scripts) == 1 {
			return "", tabOrderReadError{"script"}
		}
		return "one — window\ntwo — window\none — second pane\n", nil
	})
	if err != nil || strings.Join(got, ",") != "one,two" || len(scripts) != 2 {
		t.Fatalf("legacy compatibility: %v %v", got, err)
	}
	if !strings.Contains(scripts[0], "name of every tab of every window") {
		t.Fatal("bulk read was not used first")
	}
}

func TestTabOrderDoesNotRetryDeniedOrTimedOutRead(t *testing.T) {
	for _, reason := range []string{"automation-denied", "timeout"} {
		calls := 0
		_, err := readTabOrder(context.Background(), func(context.Context, string) (string, error) { calls++; return "", tabOrderReadError{reason} })
		if err == nil || calls != 1 {
			t.Fatalf("retried %s: %d %v", reason, calls, err)
		}
	}
}

func TestTabOrderOwnScriptProcessHonorsDeadline(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("osascript is macOS-only; pure budget/fallback tests run everywhere")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	// Local delay only: no application, automation prompt or UI action.
	_, err := runTabOrderScript(ctx, "delay 10")
	var failed tabOrderReadError
	if !errors.As(err, &failed) || failed.reason != "timeout" || time.Since(start) > 2*time.Second {
		t.Fatalf("deadline: %v after %s", err, time.Since(start))
	}
}

func TestTabOrderErrorDoesNotExposeScriptOutput(t *testing.T) {
	err := tabOrderError(errors.New("private command text"), "private terminal title (-1743)")
	if err.Error() != "tab order unavailable: automation-denied" {
		t.Fatalf("unsafe diagnostic: %v", err)
	}
}
