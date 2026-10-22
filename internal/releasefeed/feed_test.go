package releasefeed

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var testKeyOnce sync.Once
var testKey *rsa.PrivateKey

func fixture(t *testing.T) (Manifest, []byte, []byte) {
	t.Helper()
	testKeyOnce.Do(func() {
		var err error
		testKey, err = rsa.GenerateKey(rand.Reader, 3072)
		if err != nil {
			panic(err)
		}
	})
	der, err := x509.MarshalPKIXPublicKey(&testKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	public := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	m := Manifest{Schema: 1, Version: "1.2.3", SourceCommit: strings.Repeat("a", 40), InstallerSource: strings.Repeat("b", 40), PublishedAt: "2026-10-22T01:00:00Z", Assets: map[string]Artifact{}}
	for _, name := range AssetNames(m.Version) {
		h := sha256.Sum256([]byte(name))
		m.Assets[name] = Artifact{hex.EncodeToString(h[:]), int64(len(name))}
	}
	return m, public, sign(t, m)
}
func sign(t *testing.T, m Manifest) []byte {
	t.Helper()
	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	h := sha256.Sum256(payload)
	sig, err := rsa.SignPKCS1v15(rand.Reader, testKey, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(Envelope{base64.StdEncoding.EncodeToString(payload), base64.StdEncoding.EncodeToString(sig)})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func TestSignatureAndExactArtifactScope(t *testing.T) {
	m, key, body := fixture(t)
	got, err := Verify(body, key)
	if err != nil || got.Version != m.Version {
		t.Fatalf("valid: %v %v", got, err)
	}
	for _, change := range []string{"tamper", "extra", "missing", "oversize", "bad-source", "bad-version", "bad-hash"} {
		t.Run(change, func(t *testing.T) {
			m, key, body := fixture(t)
			switch change {
			case "tamper":
				var e Envelope
				_ = json.Unmarshal(body, &e)
				e.Payload = base64.StdEncoding.EncodeToString([]byte(`{"schema":1}`))
				body, _ = json.Marshal(e)
			case "extra":
				m.Assets["private.ipa"] = Artifact{strings.Repeat("a", 64), 1}
				body = sign(t, m)
			case "missing":
				delete(m.Assets, "install.sh")
				body = sign(t, m)
			case "oversize":
				m.Assets["install.sh"] = Artifact{strings.Repeat("a", 64), 1 << 21}
				body = sign(t, m)
			case "bad-source":
				m.SourceCommit = "main"
				body = sign(t, m)
			case "bad-version":
				m.Version = "1.2.3-rc.1"
				body = sign(t, m)
			case "bad-hash":
				m.Assets["install.sh"] = Artifact{"not-a-hash", 1}
				body = sign(t, m)
			}
			if _, err := Verify(body, key); !errors.Is(err, ErrInvalid) {
				t.Fatal("accepted unverified/invalid metadata", err)
			}
		})
	}
	if _, err := Verify(append(body, []byte(` {}`)...), key); err == nil {
		t.Fatal("accepted trailing JSON")
	}
	if _, err := Verify(bytes.Repeat([]byte("x"), Limit+1), key); err == nil {
		t.Fatal("accepted oversized metadata")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestFetchNeverFollowsRedirectOrForwardsCredentials(t *testing.T) {
	_, key, body := fixture(t)
	calls := 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("credentials forwarded")
		}
		if r.URL.Host == "downloads.ccy.dev" {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://evil.example/"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		if r.URL.Host != "downloads.ccy.pub" {
			t.Fatal("followed untrusted redirect")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
	})}
	if m, err := fetch(context.Background(), client, "v1.2.3", key); err != nil || m.Version != "1.2.3" || calls != 2 {
		t.Fatalf("fetch: %v %v calls=%d", m, err, calls)
	}
	if _, err := fetch(context.Background(), client, "v1.2.4", key); !errors.Is(err, ErrInvalid) {
		t.Fatal("wrong exact version accepted", err)
	}
	calls = 0
	client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	})
	if _, err := fetch(context.Background(), client, "", key); !errors.Is(err, ErrInvalid) || calls != 1 {
		t.Fatal("verification failure fell through", err, calls)
	}
}
func TestArtifactIntegrityAndOriginBounds(t *testing.T) {
	h := sha256.Sum256([]byte("verified"))
	a := Artifact{hex.EncodeToString(h[:]), 8}
	if err := CheckFile(strings.NewReader("verified"), a); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"tampered", "verified-extra", "short"} {
		if CheckFile(strings.NewReader(body), a) == nil {
			t.Fatal("bad bytes accepted")
		}
	}
	for _, raw := range []string{"http://downloads.ccy.dev/x", "https://downloads.ccy.dev:443/x", "https://downloads.ccy.dev.evil/x", "https://user@downloads.ccy.dev/x", "https://downloads.ccy.pub/x?token=x"} {
		u, _ := url.Parse(raw)
		if TrustedURL(u) {
			t.Fatal("untrusted URL", raw)
		}
	}
	if len(AssetURLs("v1.2.3", "private.ipa")) != 0 || len(AssetURLs("../x", "install.sh")) != 0 {
		t.Fatal("arbitrary artifact URL")
	}
}
func TestStockOpenSSLVerifiesTheSamePayload(t *testing.T) {
	_, public, body := fixture(t)
	var envelope Envelope
	_ = json.Unmarshal(body, &envelope)
	payload, _ := base64.StdEncoding.DecodeString(envelope.Payload)
	signature, _ := base64.StdEncoding.DecodeString(envelope.Signature)
	dir := t.TempDir()
	for name, data := range map[string][]byte{"key.pem": public, "payload": payload, "signature": signature} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("OpenSSL CLI unavailable; native verification remains covered")
	}
	cmd := exec.Command(executable, "dgst", "-sha256", "-verify", filepath.Join(dir, "key.pem"), "-signature", filepath.Join(dir, "signature"), filepath.Join(dir, "payload"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("OpenSSL compatibility: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "payload"), append(payload, 'x'), 0600); err != nil {
		t.Fatal(err)
	}
	if exec.Command(executable, "dgst", "-sha256", "-verify", filepath.Join(dir, "key.pem"), "-signature", filepath.Join(dir, "signature"), filepath.Join(dir, "payload")).Run() == nil {
		t.Fatal("OpenSSL accepted tampering")
	}
}
