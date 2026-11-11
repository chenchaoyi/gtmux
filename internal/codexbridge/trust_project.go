package codexbridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

const trustProjectLimit = 128

type trustProjectPath struct {
	Canonical string
	Seal      string
}

// A project key can be a symlink alias of an explicitly denied checkout. Resolve
// its actual directory identity only inside the connection's bounded proof worker.
func readTrustProjectPath(ctx context.Context, path string) (trustProjectPath, bool) {
	if ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 || strings.ContainsAny(path, "\x00\r\n") {
		return trustProjectPath{}, false
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || ctx.Err() != nil {
		return trustProjectPath{}, false
	}
	seal, ok := ownedTrustDirectory(canonical)
	if !ok || ctx.Err() != nil {
		return trustProjectPath{}, false
	}
	return trustProjectPath{Canonical: canonical, Seal: seal}, true
}

func trustProjectLevel(raw json.RawMessage) string {
	var value struct {
		Level string `json:"trust_level"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value.Level
}

func canonicalTrustPolicy(ctx context.Context, cwd string, proof worktreeTrustProof, projects map[string]json.RawMessage, resolve func(context.Context, string) (trustProjectPath, bool)) (string, bool) {
	if ctx.Err() != nil || len(projects) > trustProjectLimit || !trustAliasAllowed(cwd, proof, projects) {
		return "", false
	}
	// Canonical spelling alone is insufficient on case-insensitive filesystems.
	// Compare directory identities as well, including owned target ancestors.
	checkoutIDs, ancestorIDs := map[string]bool{}, map[string]bool{}
	type targetObservation struct{ Path, Seal string }
	var targets []targetObservation
	for index, target := range []string{proof.Cwd, proof.Checkout, proof.MainRoot} {
		for depth, path := 0, target; depth < 64; depth, path = depth+1, filepath.Dir(path) {
			if ctx.Err() != nil {
				return "", false
			}
			seal, ok := ownedTrustDirectory(path)
			if !ok {
				if depth == 0 {
					return "", false
				}
				break
			}
			ancestorIDs[seal] = true
			targets = append(targets, targetObservation{path, seal})
			if depth == 0 && index < 2 {
				checkoutIDs[seal] = true
			}
			if filepath.Dir(path) == path {
				break
			}
			if depth == 63 {
				return "", false
			}
		}
	}
	keys := make([]string, 0, len(projects))
	for key := range projects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	type observation struct{ Key, Level, Canonical, Seal string }
	observations := make([]observation, 0, len(keys))
	for _, key := range keys {
		if ctx.Err() != nil {
			return "", false
		}
		level := trustProjectLevel(projects[key])
		if level != "trusted" && level != "untrusted" {
			return "", false
		}
		path, ok := resolve(ctx, key)
		if !ok || path.Canonical == "" || path.Seal == "" || ctx.Err() != nil {
			return "", false
		}
		// Existing decisions for this checkout remain the backend's decision, even
		// when expressed through a different spelling of the same directory.
		if checkoutIDs[path.Seal] || path.Canonical == proof.Cwd || path.Canonical == proof.Checkout {
			return "", false
		}
		if level == "untrusted" {
			if ancestorIDs[path.Seal] {
				return "", false
			}
			for _, target := range []string{proof.Cwd, proof.Checkout, proof.MainRoot} {
				rel, err := filepath.Rel(path.Canonical, target)
				if err != nil {
					return "", false
				}
				if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return "", false
				}
			}
		}
		observations = append(observations, observation{key, level, path.Canonical, path.Seal})
	}
	raw, err := json.Marshal([]any{targets, observations})
	if err != nil || ctx.Err() != nil {
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}
