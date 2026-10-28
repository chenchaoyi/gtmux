package artifact

import (
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Links returns actual Markdown destinations, excluding code, tool arguments and HTML.
func Links(source string) []string {
	if len(source) > 4<<20 {
		return nil
	}
	tree := goldmark.DefaultParser().Parse(text.NewReader([]byte(source)))
	var links []string
	_ = ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var destination []byte
		switch n := node.(type) {
		case *ast.Link:
			destination = n.Destination
		case *ast.Image:
			destination = n.Destination
		}
		if len(destination) > 0 && len(destination) <= 4096 {
			links = append(links, html.UnescapeString(string(util.UnescapePunctuations(destination))))
		}
		return ast.WalkContinue, nil
	})
	return links
}

// LocalPath resolves a file destination, never a host URL or caller-supplied home.
func LocalPath(reference, base string) (string, bool) {
	if reference == "" || len(reference) > 4096 || strings.ContainsAny(reference, "\x00\r\n") || strings.HasPrefix(reference, "//") {
		return "", false
	}
	u, err := url.Parse(reference)
	if err != nil || u.RawQuery != "" || u.User != nil {
		return "", false
	}
	if u.Scheme != "" && !strings.EqualFold(u.Scheme, "file") {
		return "", false
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", false
	}
	path := u.Path
	if path == "" {
		return "", false
	}
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(state.Home(), path[2:])
	}
	if !filepath.IsAbs(path) {
		if base == "" || !filepath.IsAbs(base) {
			return "", false
		}
		path = filepath.Join(base, path)
	}
	if strings.ContainsAny(path, "\x00\r\n") {
		return "", false
	}
	return filepath.Clean(path), true
}

// Related reads only links in a verified Markdown snapshot and stays in its directory.
func Related(reference, parentID, session, home string) (File, error) {
	parent, err := Resolve("gtmux-file:"+parentID, session, home)
	if err != nil || parent.Kind != "markdown" || parent.Bytes > 4<<20 {
		return File{}, ErrUnavailable
	}
	_, f, err := Open(parent.ID)
	if err != nil {
		return File{}, ErrUnavailable
	}
	defer f.Close()
	content, err := io.ReadAll(io.LimitReader(f, 4<<20+1))
	if err != nil || len(content) > 4<<20 {
		return File{}, ErrUnavailable
	}
	base := filepath.Dir(parent.Source)
	target, ok := LocalPath(reference, base)
	if !ok {
		return File{}, ErrUnavailable
	}
	linked := false
	for _, ref := range Links(string(content)) {
		path, ok := LocalPath(ref, base)
		if ok && path == target {
			linked = true
			break
		}
	}
	relative, err := filepath.Rel(base, target)
	if !linked || err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return File{}, ErrUnavailable
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return File{}, ErrUnavailable
	}
	defer root.Close()
	return add(target, session, home, true, root, relative)
}
