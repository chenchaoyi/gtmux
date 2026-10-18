package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadEnforcesTheCallersInputScope(t *testing.T) {
	for _, tc := range []struct {
		name, scope, target              string
		consent, stale, noShare, revoked bool
		want                             int
	}{
		{name: "guest without consent", scope: "guest", target: "/api/upload?id=%251", want: 403},
		{name: "guest without pane", scope: "guest", consent: true, target: "/api/upload", want: 403},
		{name: "view only pane", scope: "guest", consent: true, target: "/api/upload?id=%252", want: 403},
		{name: "another links input pane", scope: "guest", consent: true, target: "/api/upload?id=%253", want: 403},
		{name: "stale grants", scope: "guest", consent: true, stale: true, target: "/api/upload?id=%251", want: 403},
		{name: "no share manager", scope: "guest", consent: true, noShare: true, target: "/api/upload?id=%251", want: 403},
		{name: "revoked guest", scope: "guest", consent: true, revoked: true, target: "/api/upload?id=%251", want: 401},
		{name: "guest with consent and own input grant", scope: "guest", consent: true, target: "/api/upload?id=%251", want: 200},
		{name: "master legacy request", scope: "master", target: "/api/upload", want: 200},
		{name: "paired device legacy request", scope: "device", target: "/api/upload", want: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enroll := NewEnrollManager(nil, nil)
			guest := enroll.MintGuest("guest", []string{"%1", "%2"}, []string{"%1"}, 0)
			enroll.MintGuest("other", []string{"%3"}, []string{"%3"}, 0)
			share := NewShareManager(ShareState{Enabled: tc.consent, Panes: []string{"%1", "%3"}, PaneEpoch: "old"}, nil)
			if tc.stale {
				share.SetEpochSource(func() string { return "new" })
			}
			if tc.noShare {
				share = nil
			}
			token := guest.Token
			if tc.revoked {
				enroll.Revoke(guest.ID)
			}
			if tc.scope == "master" {
				token = testToken
			}
			if tc.scope == "device" {
				d, _ := enroll.Redeem(enroll.Mint(), "phone")
				token = d.Token
			}
			calls := 0
			s := New(Config{Token: testToken}, Deps{Enroll: enroll, Share: share,
				Upload: func(name string, data []byte) (string, error) {
					calls++
					if name != "test.txt" || string(data) != "attachment" {
						t.Fatalf("upload changed: %q %q", name, data)
					}
					return "/uploads/test.txt", nil
				},
			})
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			// An allowed pane in the multipart body must not override a denied query.
			_ = form.WriteField("id", "%1")
			file, err := form.CreateFormFile("file", "test.txt")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = file.Write([]byte("attachment"))
			_ = form.Close()
			req := httptest.NewRequest(http.MethodPost, tc.target, &body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			s.Handler().ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rr.Code, tc.want, rr.Body.String())
			}
			if (calls == 1) != (tc.want == 200) {
				t.Fatalf("upload callback called %d times", calls)
			}
		})
	}
}
