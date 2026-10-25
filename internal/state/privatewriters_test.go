package state

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// New private-home writers must opt into policy IO. These are the remaining raw
// os writers, with their path/permission contract; an exception is not a blanket
// exemption for a package or file. Renames publish bytes prepared by these writers.
func TestRawWritersHaveAnExplicitBoundary(t *testing.T) {
	exceptions := map[string]string{
		"app/doctorlogs.go:probeStoreWrite:OpenFile":         "read-only health probe must not repair modes",
		"app/doctorlogs.go:probeDirWrite:CreateTemp":         "exclusive 0600 write probe, no existing inode rewritten",
		"app/releaseinstall.go:download:Create":              "public release asset in a fresh private installation staging directory",
		"app/releaseinstall.go:install:MkdirTemp":            "exclusive 0700 system-temporary release staging",
		"app/releaseinstall.go:installApp:MkdirAll":          "user-selected application installation directory",
		"app/releaseinstall.go:installApp:MkdirTemp":         "exclusive 0700 staging under the installation directory",
		"app/releaseinstall.go:extractReleaseCLI:OpenFile":   "exclusive 0700 executable, installer verifies the asset",
		"app/releaseinstall.go:replaceReleaseCLI:MkdirAll":   "user-selected executable installation directory",
		"app/releaseinstall.go:replaceReleaseCLI:CreateTemp": "0600 staging, then intentional executable mode 0755",
		"app/releaseinstall.go:replaceReleaseApp:MkdirTemp":  "exclusive 0700 displaced-app backup directory",
		"state/private.go:WriteForeign:WriteFile":            "foreign settings only; managed paths first route to private IO",
		"state/privateappend.go:OpenPrivateAppend:MkdirAll":  "explicit private diagnostic override; managed paths first route to private IO",
		"state/privateio.go:MkdirAll:MkdirAll":               "policy backend, foreign paths retain conventional modes",
		"state/privateio.go:OpenFile:OpenFile":               "policy backend, foreign paths and read-only inspection retain OS semantics",
		"state/privateio.go:CreateTemp:CreateTemp":           "policy backend, exclusive 0600 file after private parent validation",
		"state/privateio.go:MkdirTemp:MkdirTemp":             "policy backend, exclusive 0700 directory after private parent validation",
		"state/privatewrite.go:privateWriteParent:MkdirAll":  "only ancestors outside the managed root, never chmodded",
		"terminalkeys/gen/main.go:main:WriteFile":            "maintainer-generated source file, not runtime private state",
	}
	seen := map[string]bool{}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		osAlias := "os"
		for _, imp := range file.Imports {
			if imp.Path.Value == `"os"` && imp.Name != nil {
				osAlias = imp.Name.Name
			}
		}
		for _, decl := range file.Decls {
			fnName := "<package>"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				fnName = fn.Name.Name
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				// A post-creation chmod bypassed umask in the original upload bug.
				if (sel.Sel.Name == "Chmod" || sel.Sel.Name == "Fchmod") && len(call.Args) > 0 {
					if literal, ok := call.Args[len(call.Args)-1].(*ast.BasicLit); ok && literal.Kind == token.INT {
						bits, err := strconv.ParseUint(literal.Value, 0, 32)
						installer := strings.HasSuffix(path, "app/releaseinstall.go") && fnName == "replaceReleaseCLI"
						if err == nil && bits&0077 != 0 && !installer {
							t.Errorf("literal chmod widens permissions in %s:%s", path, fnName)
						}
					}
				}
				recv, ok := sel.X.(*ast.Ident)
				if !ok || recv.Name != osAlias {
					return true
				}
				switch sel.Sel.Name {
				case "WriteFile", "OpenFile", "Create", "Mkdir", "MkdirAll", "CreateTemp", "MkdirTemp":
				default:
					return true
				}
				key := filepath.ToSlash(strings.TrimPrefix(path, "../")) + ":" + fnName + ":" + sel.Sel.Name
				if exceptions[key] == "" {
					t.Errorf("unclassified raw writer %s: use state policy IO or document a verified external/private backend boundary", key)
				}
				seen[key] = true
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key := range exceptions {
		if !seen[key] {
			t.Errorf("stale raw-writer exception: %s", key)
		}
	}
}
