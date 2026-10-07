package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ermos/kinora/internal/stream"
)

func TestGitHubReleaseIsSlimmedAndCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.0","body":"notes","assets":[{"name":"sums.txt","browser_download_url":"x"},{"name":"kinora.apk","browser_download_url":"https://dl/kinora.apk"}]}`))
	}))
	defer srv.Close()

	rs := newReleases(false)
	rs.github = srv.URL
	req := httptest.NewRequest(http.MethodGet, "/api/v1/update", nil)
	for range 2 {
		rel, err := rs.get(req, false)
		if err != nil || rel == nil || *rel != (release{Version: "1.2.0", Notes: "notes", APK: "https://dl/kinora.apk"}) {
			t.Fatalf("release = %+v, err = %v", rel, err)
		}
	}
	if calls != 1 {
		t.Fatalf("github called %d times, want 1 (cached)", calls)
	}
	if _, err := rs.get(req, true); err != nil || calls != 2 {
		t.Fatalf("forced check: github called %d times, err = %v", calls, err)
	}
}

func TestLocalRelease(t *testing.T) {
	rs := newReleases(true)
	rs.local = t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/update", nil)
	if rel, err := rs.get(req, false); rel != nil || err != nil {
		t.Fatalf("nothing built: release = %+v, err = %v", rel, err)
	}

	meta := `{"elements":[{"versionName":"1.0.13","outputFile":"app-release.apk"}]}`
	if err := os.WriteFile(filepath.Join(rs.local, "output-metadata.json"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
	if rel, _ := rs.get(req, false); rel != nil {
		t.Fatalf("metadata without the APK: release = %+v", rel)
	}
	if err := os.WriteFile(rs.localAPK(), []byte("apk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rel, err := rs.get(req, false); err != nil || rel == nil || rel.Version != "1.0.13" {
		t.Fatalf("release = %+v, err = %v", rel, err)
	}
}

func TestLocalAPKNeedsAValidToken(t *testing.T) {
	rs := newReleases(true)
	rs.local = t.TempDir()
	if err := os.WriteFile(rs.localAPK(), []byte("apk"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &Handler{signer: stream.NewSigner([]byte("k")), releases: rs}
	valid, _ := h.signer.Sign(apkToken{Exp: time.Now().Add(time.Minute).Unix()})
	expired, _ := h.signer.Sign(apkToken{Exp: time.Now().Add(-time.Minute).Unix()})
	for tok, want := range map[string]int{valid: http.StatusOK, expired: http.StatusUnauthorized, valid + "x": http.StatusUnauthorized, "": http.StatusUnauthorized} {
		w := httptest.NewRecorder()
		h.localAPK(w, httptest.NewRequest(http.MethodGet, "/api/v1/update/apk?token="+url.QueryEscape(tok), nil))
		if w.Code != want {
			t.Errorf("token %q: status %d, want %d", tok, w.Code, want)
		}
	}
}
