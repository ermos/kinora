// Package stream relays upstream media through the server: CDNs want a Referer/User-Agent a browser
// cannot send cross-origin, and most of them do not allow CORS. HLS playlists are rewritten so every
// variant, segment and key goes through the proxy too.
package stream

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ermos/kinora/internal/safehttp"
	"github.com/ermos/kinora/internal/scraper"
)

// Signer signs opaque tokens so clients can only reach URLs the server handed out.
type Signer struct{ key []byte }

func NewSigner(key []byte) *Signer { return &Signer{key: key} }

func (s *Signer) Sign(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	p := base64.RawURLEncoding.EncodeToString(b)
	return p + "." + s.mac(p), nil
}

func (s *Signer) Verify(token string, v any) error {
	p, sig, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.mac(p))) {
		return errors.New("invalid token")
	}
	b, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (s *Signer) mac(p string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(p))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
}

// target is what a proxy URL carries. The URL itself is the credential (no cookie needed): native and TV
// players (ExoPlayer, AVPlayer, AirPlay, Chromecast) do not send the app's cookies.
type target struct {
	URL     string            `json:"u"`
	Headers map[string]string `json:"h,omitempty"`
	Expires int64             `json:"e"`
	// Rate caps the stream in bytes per second (0: unlimited), shared by every request carrying the same Stream.
	Rate   int64  `json:"r,omitempty"`
	Stream string `json:"s,omitempty"`
}

// URLTTL bounds how long a proxied URL works, enough for a long movie with pauses.
const URLTTL = 12 * time.Hour

type Proxy struct {
	signer   *Signer
	prefix   string // public path of the proxy handler, e.g. /api/v1/proxy
	client   *http.Client
	limiters limiters
}

func NewProxy(signer *Signer, prefix string) *Proxy {
	// No overall timeout: a movie file can stream for hours. The dial and header phases are bounded. Stream and
	// playlist URLs come from third-party sites: only public addresses are reached.
	tr := safehttp.Transport()
	tr.ResponseHeaderTimeout = 20 * time.Second
	return &Proxy{signer: signer, prefix: prefix, client: &http.Client{Transport: tr}}
}

// URL returns the proxied URL for a resolved stream, capped at maxMbps Mbit/s (0: unlimited).
func (p *Proxy) URL(st scraper.Stream, maxMbps int) (string, error) {
	t := target{URL: st.URL, Headers: st.Headers, Expires: time.Now().Add(URLTTL).Unix()}
	if maxMbps > 0 {
		id := make([]byte, 9)
		_, _ = rand.Read(id)
		t.Rate, t.Stream = int64(maxMbps)*1_000_000/8, base64.RawURLEncoding.EncodeToString(id)
	}
	return p.sign(t)
}

func (p *Proxy) sign(t target) (string, error) {
	tok, err := p.signer.Sign(t)
	if err != nil {
		return "", err
	}
	return p.prefix + "?t=" + tok, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var t target
	if err := p.signer.Verify(r.URL.Query().Get("t"), &t); err != nil || time.Now().Unix() > t.Expires {
		http.Error(w, "invalid or expired link", http.StatusForbidden)
		return
	}
	if t.Rate > 0 {
		w = throttled{ResponseWriter: w, ctx: r.Context(), l: p.limiters.get(t.Stream, float64(t.Rate))}
	}
	if f, ok := scraper.ParseAbyss(t.URL); ok {
		p.serveAbyss(w, r, f, t.Headers)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, t.URL, nil)
	if err != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
		http.Error(w, "invalid upstream URL", http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", scraper.UserAgent)
	for k, v := range t.Headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	if rg := r.Header.Get("Range"); rg != "" {
		req.Header.Set("Range", rg)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		slog.Warn("proxy upstream failed", "err", err) // not echoed: it would tell what answers on the network
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		http.Error(w, "upstream status "+resp.Status, http.StatusBadGateway)
		return
	}
	if isPlaylist(resp, t.URL) {
		p.rewrite(w, resp, t)
		return
	}
	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func isPlaylist(resp *http.Response, u string) bool {
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.Contains(ct, "mpegurl") || strings.HasSuffix(strings.SplitN(u, "?", 2)[0], ".m3u8")
}

var reURI = regexp.MustCompile(`URI="([^"]+)"`)

// rewrite points every URI of an HLS playlist back to the proxy, resolved against the playlist's final URL.
func (p *Proxy) rewrite(w http.ResponseWriter, resp *http.Response, t target) {
	base := resp.Request.URL
	proxied := func(ref string) string {
		u, err := base.Parse(strings.TrimSpace(ref))
		if err != nil {
			return ref
		}
		out, err := p.sign(target{URL: u.String(), Headers: t.Headers, Expires: t.Expires, Rate: t.Rate, Stream: t.Stream})
		if err != nil {
			return ref
		}
		return out
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 5<<20))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "#"):
			line = reURI.ReplaceAllStringFunc(line, func(m string) string {
				return `URI="` + proxied(reURI.FindStringSubmatch(m)[1]) + `"`
			})
		case strings.TrimSpace(line) != "":
			line = proxied(line)
		}
		_, _ = io.WriteString(w, line+"\n")
	}
}

// Kind tells the player how to read a stream URL.
func Kind(u string) string {
	if pu, err := url.Parse(u); err == nil && strings.HasSuffix(strings.ToLower(pu.Path), ".m3u8") {
		return "hls"
	}
	return "file"
}
