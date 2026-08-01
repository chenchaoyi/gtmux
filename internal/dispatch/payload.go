package dispatch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// The SHELL-FREE payload channel.
//
// `gtmux spawn <goal…>` / `gtmux send <pane> <text…>` take their payload as argv, which
// means the caller must hand a shell a safe rendering of an arbitrary natural-language
// instruction. That is a guarantee no caller can hold: a long enough goal eventually
// contains a backtick, a `$`, a quote or a newline, and inside double quotes a shell
// EXECUTES the backticked span (the 2026-08-01 incident: a dispatch died on
// `command substitution: syntax error near unexpected token 'done'`, having been
// recorded as a footgun twice already). Reading the payload from a FILE removes the
// shell from the path entirely — the bytes go file → gtmux → `tmux load-buffer -` (a
// pipe, byte-exact) → the agent's input box.
//
// Exactly ONE normalization is applied, and it is specified rather than incidental: at
// most one trailing newline is stripped, because every heredoc and `printf` appends one
// and nobody means it as part of the instruction.

// PayloadMax bounds a payload file. A dispatch is an instruction typed into an agent's
// input box, not a data transfer — past this size the caller almost certainly meant a
// file path IN the goal, and `--goal-file /dev/zero` should fail rather than stream
// forever.
const PayloadMax = 256 << 10 // 256 KiB

// ErrPayloadEmpty is returned for a payload that holds no non-whitespace content —
// dispatching it would submit an empty turn.
var ErrPayloadEmpty = errors.New("payload is empty")

// ReadPayload reads a dispatch payload from path, or from stdin when path is "-".
// The content is returned VERBATIM apart from a single stripped trailing newline
// (and its CR, for a CRLF file). Callers pass os.Stdin in production; tests pass a
// reader.
func ReadPayload(path string, stdin io.Reader) (string, error) {
	var (
		r   io.Reader
		err error
	)
	if path == "-" {
		r = stdin
		if r == nil {
			return "", errors.New("no standard input to read the payload from")
		}
	} else {
		f, ferr := os.Open(path)
		if ferr != nil {
			return "", ferr
		}
		defer f.Close()
		r = f
	}
	// Read one byte past the cap so an oversize payload is REPORTED, not truncated into
	// a silently mutilated instruction.
	b, err := io.ReadAll(io.LimitReader(r, PayloadMax+1))
	if err != nil {
		return "", err
	}
	if len(b) > PayloadMax {
		return "", fmt.Errorf("payload is larger than %d bytes", PayloadMax)
	}
	s := TrimPayload(string(b))
	if strings.TrimSpace(s) == "" {
		return "", ErrPayloadEmpty
	}
	return s, nil
}

// TrimPayload applies the ONE documented normalization: strip at most one trailing
// newline (with its carriage return). Interior newlines, leading whitespace, and every
// other byte are preserved exactly. Exported so the contract is testable on its own.
func TrimPayload(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return strings.TrimSuffix(s, "\r")
}
