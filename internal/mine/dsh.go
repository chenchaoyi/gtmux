package mine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

func dshIdentity(path string) (dsh.Session, error) {
	// The header's ID may contain escapes, so resolve the selected artifact by its
	// parent segment through discovery. Never mine historical generations or copies.
	sessions, err := dsh.Sessions.List()
	if err != nil {
		return dsh.Session{}, err
	}
	for _, s := range sessions {
		if filepath.Clean(s.Path) == filepath.Clean(path) {
			return s, nil
		}
	}
	return dsh.Session{}, os.ErrNotExist
}
func readDSH(path string, start int64, o readOpts) (readResult, int64, error) {
	session, err := dshIdentity(path)
	if err != nil {
		return readResult{}, start, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return readResult{}, start, err
	}
	c := newConvo(o, func(name string) bool { return name == "bash" || name == "shell" })
	c.meta(session.ID, session.Cwd)
	cursor := dsh.Cursor{File: fi, Offset: start}
	err = dsh.Scan(path, &cursor, func() {
		c = newConvo(readOpts{policySince: o.policySince, sinceUnix: o.sinceUnix, machine: o.machine}, c.shell)
		c.meta(session.ID, session.Cwd)
	}, func(line []byte) error {
		var e dsh.Event
		if err := json.Unmarshal(line, &e); err != nil {
			return err
		}
		if e.Type == "session" || e.Time/1000 < o.policySince {
			return nil
		}
		c.sourceFile, c.sourceOffset = path, cursor.Offset
		var data struct {
			Role, ID, Name, CallID string
			Source                 struct{ Kind string }
			Content                []struct{ Type, Text string }
			Message                struct {
				Role, ToolCallID string
				Content          []struct{ Type, Text string }
			}
		}
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return err
		}
		text := func(blocks []struct{ Type, Text string }) string {
			var xs []string
			for _, b := range blocks {
				if b.Type == "text" {
					xs = append(xs, b.Text)
				}
			}
			return strings.Join(xs, "\n")
		}
		switch e.Type {
		case "user/message":
			if data.Role == "user" && data.Source.Kind == "user" {
				c.human(text(data.Content), e.Time/1000, fmt.Sprintf("dsh:%s:%d", session.HomeID, e.Seq))
			}
		case "assistant/message":
			if data.Message.Role == "assistant" {
				c.assistant(text(data.Message.Content))
			}
		case "tool/call":
			c.toolCall(data.CallID, data.Name)
		case "tool/result":
			c.toolResult(data.Message.ToolCallID, text(data.Message.Content), e.Time/1000)
		}
		return nil
	})
	return c.finish(), cursor.Offset, err
}
