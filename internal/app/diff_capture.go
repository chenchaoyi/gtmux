package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Read a bounded prefix, then stop the producer. The timeout also abandons an
// unreapable child, following the radar sampler's existing process boundary.
func commandPrefix(timeout time.Duration, limit int, name string, args ...string) (string, bool, error) {
	if timeout <= 0 {
		return "", false, context.DeadlineExceeded
	}
	cmd := exec.Command(name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", false, err
	}
	if err := cmd.Start(); err != nil {
		return "", false, err
	}
	type result struct {
		body string
		cut  bool
		err  error
	}
	done := make(chan result, 1)
	go func() {
		body, readErr := io.ReadAll(io.LimitReader(stdout, int64(limit)+1))
		cut := len(body) > limit
		if cut {
			body = body[:limit]
			_ = cmd.Process.Kill()
			_ = stdout.Close()
		}
		waitErr := cmd.Wait()
		if readErr == nil && !cut {
			readErr = waitErr
		}
		done <- result{string(body), cut, readErr}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.body, r.cut, r.err
	case <-timer.C:
		_ = cmd.Process.Kill()
		_ = stdout.Close()
		return "", false, fmt.Errorf("%s output timed out: %w", name, context.DeadlineExceeded)
	}
}

func diffForCwd(cwd string) (string, error) {
	const timeout = 8 * time.Second
	deadline := time.Now().Add(timeout)
	if _, _, err := commandPrefix(timeout, 32, "git", "-C", cwd, "rev-parse", "--is-inside-work-tree"); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", nil
	}
	var b strings.Builder
	cut := false
	write := func(s string) {
		remaining := maxDiffBytes - b.Len()
		if len(s) > remaining {
			s = s[:remaining]
			cut = true
		}
		b.WriteString(s)
	}
	git := func(args ...string) (string, error) {
		out, clipped, err := commandPrefix(time.Until(deadline), maxDiffBytes-b.Len(), "git", append([]string{"-C", cwd, "-c", "color.ui=never"}, args...)...)
		cut = cut || clipped
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return out, nil
	}
	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if branch = strings.TrimSpace(branch); branch != "" {
		write("# branch " + branch + "\n")
	}
	diff, err := git("diff", "HEAD")
	if err != nil {
		return "", err
	}
	write(diff)
	if !cut {
		untracked, err := git("ls-files", "--others", "--exclude-standard")
		if err != nil {
			return "", err
		}
		if untracked = strings.TrimSpace(untracked); untracked != "" {
			write("\n# untracked:\n")
			for untracked != "" && b.Len() < maxDiffBytes {
				line, rest, _ := strings.Cut(untracked, "\n")
				untracked = rest
				write("+ " + line + "\n")
			}
			cut = cut || untracked != ""
		}
	}
	if cut {
		b.WriteString("\n… (diff truncated)\n")
	}
	return b.String(), nil
}
