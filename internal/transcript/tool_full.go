package transcript

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

var ErrToolUnavailable = errors.New("tool input unavailable; reload the conversation")

func toolInputRef(source string, off int64, ordinal int, s Step) string {
	sum := sha256.Sum256([]byte(s.Title + "\x00" + s.fullInput + "\x00" + s.fullWorkdir))
	return fmt.Sprintf("%s:%d:%d:%x", source, off, ordinal, sum[:12])
}

// ToolInput reads one original call on demand. Normal transcript polls/cache retain
// bounded previews. A reference is scoped to the resolved session's source files;
// it cannot select a path, a result, context, or a record from another session.
func ToolInput(agent, sessionID, ref string) (Step, error) {
	if len(ref) > 160 {
		return Step{}, ErrToolUnavailable
	}
	parts := strings.Split(ref, ":")
	if len(parts) != 4 {
		return Step{}, ErrToolUnavailable
	}
	off, e1 := strconv.ParseInt(parts[1], 10, 64)
	ordinal, e2 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || off < 0 || ordinal < 0 {
		return Step{}, ErrToolUnavailable
	}
	if normalizeAgent(agent) == "dsh" {
		session, err := dsh.Sessions.Resolve(sessionID)
		if err != nil || recordSourceID(session.Path) != parts[0] {
			return Step{}, ErrToolUnavailable
		}
		var cursor dsh.Cursor
		var found Step
		stop := errors.New("found")
		err = dsh.Scan(session.Path, &cursor, func() {}, func(line []byte) error {
			st := &parseState{sourceID: parts[0], fullTools: true}
			if err := dshStep(line, st); err != nil {
				return err
			}
			if st.off != off {
				return nil
			}
			if value, ok := matchingTool(st, ref); ok {
				found = value
				return stop
			}
			return nil
		})
		if err == stop {
			return found, nil
		}
		return Step{}, ErrToolUnavailable
	}
	path, parser := resolveLog(agent, sessionID)
	paths := []string{path}
	if normalizeAgent(agent) == "codex" {
		paths = codexLogPathsFresh(sessionID)
		parser = codexStep
	}
	for _, path := range paths {
		if path == "" || parser == nil || recordSourceID(path) != parts[0] {
			continue
		}
		return readToolInput(path, parser, off, ref)
	}
	return Step{}, ErrToolUnavailable
}

func matchingTool(st *parseState, ref string) (Step, bool) {
	if st.cur != nil {
		for _, segment := range st.cur.Segments {
			for _, step := range segment.Steps {
				if step.InputRef == ref {
					return step, true
				}
			}
		}
	}
	return Step{}, false
}

func readToolInput(path string, parser stepFn, off int64, ref string) (Step, error) {
	f, err := os.Open(path)
	if err != nil {
		return Step{}, ErrToolUnavailable
	}
	defer f.Close()
	// A bounded record allocation, never a silent content cut. Reject incomplete,
	// oversized or rewritten records explicitly rather than presenting a partial input.
	const maxRecord = 32 << 20
	if off > 0 {
		var prev [1]byte
		if _, err := f.ReadAt(prev[:], off-1); err != nil || prev[0] != '\n' {
			return Step{}, ErrToolUnavailable
		}
	}
	line, err := bufio.NewReader(io.NewSectionReader(f, off, maxRecord+1)).ReadString('\n')
	if err != nil || len(line) > maxRecord {
		return Step{}, ErrToolUnavailable
	}
	st := &parseState{sourceID: recordSourceID(path), off: off, fullTools: true}
	parser(line, st)
	if step, ok := matchingTool(st, ref); ok {
		return step, nil
	}
	return Step{}, ErrToolUnavailable
}
