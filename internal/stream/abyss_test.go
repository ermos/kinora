package stream

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ermos/kinora/internal/scraper"
)

// An Abyss file served by parts with an encrypted head must come out of the proxy as the plain file,
// for ranges crossing the encrypted head and the part boundary.
func TestProxyServesAbyss(t *testing.T) {
	const part = 100_000
	plain := make([]byte, 200_000)
	for i := range plain {
		plain[i] = byte(i * 7)
	}
	upstream := httptest.NewServer(nil)
	defer upstream.Close()
	f := scraper.AbyssFile{Size: int64(len(plain)), URL: upstream.URL + "/x/abc.200000.3", Part: part}

	stored := bytes.Clone(plain)
	head, n := f.Head(0)
	head.XORKeyStream(stored[:n], stored[:n])
	upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() != "https://site/" {
			http.Error(w, "no referer", http.StatusForbidden)
			return
		}
		parts := map[string][]byte{"/x/abc.200000.3": stored[:part], "/x/abc.200000.31": stored[part:]}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(parts[r.URL.Path]))
	})

	p := newTestProxy()
	b, _ := json.Marshal(f)
	u, _ := p.URL(scraper.Stream{URL: "abyss:" + base64.RawURLEncoding.EncodeToString(b), Headers: map[string]string{"Referer": "https://site/"}}, 0)

	for _, rg := range [][2]int{{0, 199_999}, {65_530, 100_010}, {150_000, 199_999}} {
		req := httptest.NewRequest(http.MethodGet, u, nil)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rg[0], rg[1]))
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusPartialContent || !bytes.Equal(rec.Body.Bytes(), plain[rg[0]:rg[1]+1]) {
			t.Errorf("range %v: status %d, %d bytes, content differs", rg, rec.Code, rec.Body.Len())
		}
	}
}

// An upstream error page must not end up in the video: the body stops short instead.
func TestServeAbyssStopsOnUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "<html>blocked</html>") // 200, ignoring the Range header
	}))
	defer upstream.Close()
	p := newTestProxy()
	b, _ := json.Marshal(scraper.AbyssFile{Size: 1000, URL: upstream.URL + "/x/f", Part: 1000, Sora: true})
	u, _ := p.URL(scraper.Stream{URL: "abyss:" + base64.RawURLEncoding.EncodeToString(b)}, 0)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	if rec.Body.Len() != 0 {
		t.Fatalf("body %q, want nothing", rec.Body.String())
	}
}
