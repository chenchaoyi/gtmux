package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fcmPusher struct {
	project, email    string
	key               *rsa.PrivateKey
	client            *http.Client
	oauthURL, sendURL string
	mu                sync.Mutex
	token             string
	until             time.Time
}
type fcmFailure struct {
	Status int
	Code   string
}

func (e *fcmFailure) Error() string { return fmt.Sprintf("fcm status %d", e.Status) }

func fcmFromEnv() (*fcmPusher, error) {
	path := os.Getenv("FCM_SERVICE_ACCOUNT_PATH")
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot read FCM service account")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	if err != nil || len(data) > 64<<10 {
		return nil, errors.New("invalid FCM service account")
	}
	return newFCMPusher(data)
}
func newFCMPusher(data []byte) (*fcmPusher, error) {
	var credentials struct {
		Type    string `json:"type"`
		Project string `json:"project_id"`
		Email   string `json:"client_email"`
		Key     string `json:"private_key"`
	}
	if json.Unmarshal(data, &credentials) != nil || credentials.Type != "service_account" || !regexp.MustCompile(`^[a-z][a-z0-9-]{4,62}$`).MatchString(credentials.Project) || !regexp.MustCompile(`^[^\s@]+@[^\s@]+\.gserviceaccount\.com$`).MatchString(credentials.Email) {
		return nil, errors.New("invalid FCM configuration")
	}
	block, rest := pem.Decode([]byte(credentials.Key))
	if block == nil || block.Type != "PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("invalid FCM key")
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid FCM key")
	}
	key, ok := value.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < 2048 {
		return nil, errors.New("FCM requires RSA 2048 or stronger")
	}
	return &fcmPusher{project: credentials.Project, email: credentials.Email, key: key, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, oauthURL: "https://oauth2.googleapis.com/token", sendURL: "https://fcm.googleapis.com/v1/projects/" + credentials.Project + "/messages:send"}, nil
}
func fcmMessage(req pushRequest) (map[string]any, error) {
	if req.LiveActivity || req.Token == "" || len(req.Token) > 4096 {
		return nil, errors.New("invalid FCM intent")
	}
	data := map[string]string{"title": req.Title, "body": req.Body, "server": req.Subtitle, "pane": req.Pane, "kind": req.Kind, "silent": strconv.FormatBool(req.Silent)}
	if req.Options != nil {
		data["options"] = strconv.Itoa(*req.Options)
	}
	if req.Badge != nil {
		data["badge"] = strconv.Itoa(*req.Badge)
	}
	if req.CollapseID != "" {
		data["collapseId"] = req.CollapseID
	}
	priority, ttl := "HIGH", "120s"
	if req.Silent {
		priority, ttl = "NORMAL", "300s"
	}
	message := map[string]any{"message": map[string]any{"token": req.Token, "data": data, "android": map[string]string{"priority": priority, "ttl": ttl}}}
	encoded, _ := json.Marshal(message)
	if len(encoded) > 4096 {
		return nil, errors.New("FCM payload too large")
	}
	return message, nil
}
func (p *fcmPusher) accessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Until(p.until) > time.Minute {
		return p.token, nil
	}
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": p.email, "scope": "https://www.googleapis.com/auth/firebase.messaging", "aud": "https://oauth2.googleapis.com/token", "iat": now, "exp": now + 3600})
	input := b64(header) + "." + b64(claims)
	hash := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, hash[:])
	if err != nil {
		return "", errors.New("FCM signing failed")
	}
	body := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {input + "." + b64(signature)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.oauthURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := p.client.Do(req)
	if err != nil {
		return "", errors.New("FCM authorization unavailable")
	}
	defer response.Body.Close()
	var value struct {
		Token   string `json:"access_token"`
		Type    string `json:"token_type"`
		Expires int    `json:"expires_in"`
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil || len(data) > 16384 || response.StatusCode != 200 || json.Unmarshal(data, &value) != nil || value.Token == "" || len(value.Token) > 8192 || value.Type != "Bearer" || value.Expires < 60 {
		return "", errors.New("invalid FCM authorization response")
	}
	p.token = value.Token
	p.until = time.Now().Add(time.Duration(min(3600, value.Expires)) * time.Second)
	return p.token, nil
}
func (p *fcmPusher) Push(ctx context.Context, req pushRequest) error {
	message, err := fcmMessage(req)
	if err != nil {
		return err
	}
	token, err := p.accessToken(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(message)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.sendURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return errors.New("FCM gateway unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(response.Body, 16384))
	var value struct {
		Error struct {
			Details []struct {
				Code string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(data, &value)
	code := "REJECTED"
	for _, d := range value.Error.Details {
		if d.Code == "UNREGISTERED" {
			code = d.Code
		}
	}
	return &fcmFailure{Status: response.StatusCode, Code: code}
}
