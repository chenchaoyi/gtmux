// Package releasefeed verifies public release metadata independently of its host.
package releasefeed

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const Limit = 64 << 10
const AssetLimit = 256 << 20

//go:embed public.pem
var PublicKey []byte

var Origins = []string{"https://downloads.ccy.dev", "https://downloads.ccy.pub"}
var ErrInvalid = errors.New("release manifest verification failed")
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// IsStableVersion reports whether a release can have a signed stable manifest.
func IsStableVersion(version string) bool {
	return versionPattern.MatchString(strings.TrimPrefix(version, "v"))
}

type Artifact struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Manifest struct {
	Schema          int                 `json:"schema"`
	Version         string              `json:"version"`
	SourceCommit    string              `json:"source_commit"`
	InstallerSource string              `json:"installer_source"`
	PublishedAt     string              `json:"published_at"`
	Assets          map[string]Artifact `json:"assets"`
}

type Envelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func strictJSON(data []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	return nil
}

// Verify authenticates the original payload bytes before interpreting metadata.
// Unsigned hashes from a mirror are never a replacement for this trust anchor.
func Verify(data, publicPEM []byte) (*Manifest, error) {
	if len(data) > Limit {
		return nil, ErrInvalid
	}
	var envelope Envelope
	if strictJSON(data, &envelope) != nil {
		return nil, ErrInvalid
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(envelope.Payload)
	if err != nil || len(payload) == 0 || len(payload) > Limit {
		return nil, ErrInvalid
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature)
	if err != nil {
		return nil, ErrInvalid
	}
	block, rest := pem.Decode(publicPEM)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrInvalid
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalid
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok || key.N.BitLen() < 3072 {
		return nil, ErrInvalid
	}
	digest := sha256.Sum256(payload)
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return nil, ErrInvalid
	}
	var m Manifest
	if strictJSON(payload, &m) != nil || m.Schema != 1 || !versionPattern.MatchString(m.Version) || !commitPattern.MatchString(m.SourceCommit) || !commitPattern.MatchString(m.InstallerSource) {
		return nil, ErrInvalid
	}
	if _, err := time.Parse(time.RFC3339, m.PublishedAt); err != nil {
		return nil, ErrInvalid
	}
	names := AssetNames(m.Version)
	if len(m.Assets) != len(names) {
		return nil, ErrInvalid
	}
	for _, name := range names {
		a, ok := m.Assets[name]
		if !ok || !hashPattern.MatchString(a.SHA256) || a.Size <= 0 || a.Size > AssetLimit {
			return nil, ErrInvalid
		}
		if (name == "install.sh" || name == "SHASUMS256.txt") && a.Size > 1<<20 {
			return nil, ErrInvalid
		}
	}
	return &m, nil
}

func AssetNames(version string) []string {
	var names []string
	for _, platform := range []string{"darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			names = append(names, fmt.Sprintf("gtmux-%s-%s-%s.tar.gz", version, platform, arch))
		}
	}
	return append(names, "Gtmux-"+version+"-macos.zip", "SHASUMS256.txt", "install.sh")
}

// TrustedURL excludes credentials, ports, queries and cross-origin redirects.
func TrustedURL(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return false
	}
	for _, origin := range Origins {
		if u.Scheme+"://"+u.Host == origin {
			return true
		}
	}
	return false
}

func AssetURLs(version, name string) []string {
	version = strings.TrimPrefix(version, "v")
	if !versionPattern.MatchString(version) {
		return nil
	}
	valid := false
	for _, candidate := range AssetNames(version) {
		if name == candidate {
			valid = true
			break
		}
	}
	if !valid {
		return nil
	}
	out := make([]string, 0, len(Origins))
	for _, origin := range Origins {
		out = append(out, origin+"/gtmux/v"+version+"/"+name)
	}
	return out
}

// Fetch accepts either the stable pointer or an exact immutable version. It does
// not forward credentials or follow redirects; a verified wrong version fails.
func Fetch(ctx context.Context, client *http.Client, version string) (*Manifest, error) {
	return fetch(ctx, client, version, PublicKey)
}

func fetch(ctx context.Context, client *http.Client, version string, key []byte) (*Manifest, error) {
	suffix := "/gtmux/stable.json"
	if version != "" {
		version = strings.TrimPrefix(version, "v")
		if !versionPattern.MatchString(version) {
			return nil, ErrInvalid
		}
		suffix = "/gtmux/v" + version + "/manifest.json"
	}
	if client == nil {
		client = http.DefaultClient
	}
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	var last error
	for _, origin := range Origins {
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		req, _ := http.NewRequestWithContext(requestCtx, http.MethodGet, origin+suffix, nil)
		req.Header.Set("User-Agent", "gtmux-release-check")
		resp, err := c.Do(req)
		if err != nil {
			last = err
			cancel()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			last = fmt.Errorf("release source HTTP %d", resp.StatusCode)
			resp.Body.Close()
			cancel()
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, Limit+1))
		resp.Body.Close()
		cancel()
		if err != nil {
			last = err
			continue
		}
		m, err := Verify(body, key)
		if err != nil || version != "" && m.Version != version {
			return nil, ErrInvalid
		}
		return m, nil
	}
	if last == nil {
		last = errors.New("release source unavailable")
	}
	return nil, last
}

func CheckFile(reader io.Reader, expected Artifact) error {
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(reader, expected.Size+1))
	if err != nil {
		return err
	}
	if n != expected.Size || hex.EncodeToString(h.Sum(nil)) != expected.SHA256 {
		return errors.New("release asset size or SHA256 mismatch")
	}
	return nil
}
