package stream

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ermos/kinora/internal/scraper"
)

func TestSignerRejectsTampering(t *testing.T) {
	s := NewSigner([]byte("k"))
	tok, _ := s.Sign(target{URL: "https://a"})
	var got target
	if err := s.Verify(tok, &got); err != nil || got.URL != "https://a" {
		t.Fatalf("Verify = %v, %+v", err, got)
	}
	payload, sig, _ := strings.Cut(tok, ".")
	if s.Verify(payload+"x."+sig, &got) == nil || NewSigner([]byte("other")).Verify(tok, &got) == nil {
		t.Fatal("tampered or foreign token accepted")
	}
}

// The proxy must rewrite every URI of a playlist to itself, forward the stream headers upstream,
// and resolve relative URIs against the playlist URL.
func TestProxyRewritesPlaylist(t *testing.T) {
	var gotReferer string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v/master.m3u8":
			gotReferer = r.Referer()
			_, _ = io.WriteString(w, "#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,URI=\"audio/fr.m3u8\"\n#EXT-X-STREAM-INF:BANDWIDTH=1\nhd/index.m3u8\n")
		case "/v/hd/seg1.ts":
			_, _ = io.WriteString(w, "SEGMENT")
		case "/v/broken.m3u8":
			http.Error(w, "down", 522)
		}
	}))
	defer upstream.Close()

	p := newTestProxy()
	get := func(u string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
		return rec
	}
	master, _ := p.URL(scraper.Stream{URL: upstream.URL + "/v/master.m3u8", Headers: map[string]string{"Referer": "https://site/"}}, 0)
	rec := get(master)
	if rec.Code != http.StatusOK || gotReferer != "https://site/" {
		t.Fatalf("status %d, referer %q", rec.Code, gotReferer)
	}

	var uris []string
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if i := strings.Index(line, `URI="`); i >= 0 {
			uris = append(uris, strings.TrimSuffix(line[i+5:], `"`))
		} else if line != "" && !strings.HasPrefix(line, "#") {
			uris = append(uris, line)
		}
	}
	if len(uris) != 2 {
		t.Fatalf("expected 2 rewritten URIs, got %q", rec.Body.String())
	}
	var tg target
	for i, want := range []string{"/v/audio/fr.m3u8", "/v/hd/index.m3u8"} {
		tok := strings.TrimPrefix(uris[i], "/proxy?t=")
		if err := p.signer.Verify(tok, &tg); err != nil || tg.URL != upstream.URL+want || tg.Headers["Referer"] != "https://site/" {
			t.Fatalf("uri %d: %v %+v", i, err, tg)
		}
	}

	seg, _ := p.URL(scraper.Stream{URL: upstream.URL + "/v/hd/seg1.ts"}, 0)
	if rec := get(seg); rec.Body.String() != "SEGMENT" {
		t.Fatalf("segment body %q", rec.Body.String())
	}
	expired, _ := p.sign(target{URL: upstream.URL + "/v/hd/seg1.ts", Expires: time.Now().Add(-time.Minute).Unix()})
	if rec := get(expired); rec.Code != http.StatusForbidden {
		t.Fatalf("expired link should be 403, got %d", rec.Code)
	}
	broken, _ := p.URL(scraper.Stream{URL: upstream.URL + "/v/broken.m3u8"}, 0)
	if rec := get(broken); rec.Code != http.StatusBadGateway {
		t.Fatalf("upstream error should be 502, got %d", rec.Code)
	}
}

// A capped stream is paced across all its requests: two parallel ranges share the cap, an uncapped one is not slowed.
func TestProxyCapsStream(t *testing.T) {
	body := strings.Repeat("x", 50_000)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer upstream.Close()
	p := newTestProxy()
	fetch := func(u string) {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
		if rec.Body.String() != body {
			t.Errorf("body of %d bytes, want %d", rec.Body.Len(), len(body))
		}
	}

	capped, _ := p.URL(scraper.Stream{URL: upstream.URL + "/v.mp4"}, 1) // 125 kB/s
	start := time.Now()
	done := make(chan struct{})
	go func() { fetch(capped); close(done) }()
	fetch(capped)
	<-done
	if d := time.Since(start); d < 600*time.Millisecond { // 100 kB at 125 kB/s, the last 17 kB slice leaves at 0.66 s
		t.Errorf("two capped requests took %v, want about 0.66s", d)
	}

	free, _ := p.URL(scraper.Stream{URL: upstream.URL + "/v.mp4"}, 0)
	start = time.Now()
	fetch(free)
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Errorf("uncapped request took %v", d)
	}
}

// newTestProxy reaches the loopback httptest servers the production transport refuses.
func newTestProxy() *Proxy {
	p := NewProxy(NewSigner([]byte("k")), "/proxy")
	p.client = &http.Client{}
	return p
}

// Upstream URLs must be http(s) on a public address, whatever the playlist says.
func TestProxyRefusesPrivateUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "internal")
	}))
	defer upstream.Close()
	p := NewProxy(NewSigner([]byte("k")), "/proxy")
	for _, u := range []string{upstream.URL + "/admin", "file:///etc/passwd"} {
		tok, _ := p.URL(scraper.Stream{URL: u}, 0)
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tok, nil))
		if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "internal") {
			t.Errorf("%s: status %d, body %q", u, rec.Code, rec.Body.String())
		}
	}
}
