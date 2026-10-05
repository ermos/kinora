package scraper

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// More ports of vStream hosters. Match lists come from cHosterGui.checkHoster.
func init() {
	Hosters = append(Hosters,
		// The JW player family (XFS based sites), often behind p.a.c.k.e.r: one generic resolver.
		Hoster{Name: "Filelions", Match: []string{"filelions", "vidhide", "minochinos", "earnvids", "nejma", "shoooot"}, Resolve: packedPlayer},
		Hoster{Name: "Streamhide", Match: []string{"streamhide", "guccihide", "wishonly"}, Resolve: packedPlayer},
		Hoster{Name: "Streamwish", Match: []string{"streamwish", "embedwish", "warda", "swish", "hanerix", "hgcloud"}, Resolve: packedPlayer},
		Hoster{Name: "Savefiles", Match: []string{"savefiles", "streamhls"}, Resolve: savefiles},
		Hoster{Name: "Voe", Match: []string{"voe", "jamessoundcost", "magasavor", "sandratableother", "alejandrocenturyoil"}, Resolve: voe},
		Hoster{Name: "Vidmoly", Match: []string{"vidmoly"}, Resolve: vidmoly},
		Hoster{Name: "Filemoon", Match: []string{"filemoon", "xcoic", "filmoon"}, Resolve: filemoon},
		Hoster{Name: "Mixdrop", Match: []string{"mixdrop", "mixdrp", "mdbekjwqa", "mdfx9dc8n", "mxdrop"}, Resolve: mixdrop},
		Hoster{Name: "Streamtape", Match: []string{"streamtape", "tapepops"}, Resolve: streamtape},
		Hoster{Name: "Veev", Match: []string{"veev"}, Resolve: veev},
	)
}

var packedPatterns = []string{
	`sources:\s*\[\s*\{\s*file:\s*["']([^"']+)["']`,
	`"hls2":"([^"]+)"`,
	`"hls4":"([^"]+)"`,
	`file:\s*"([^"]+\.(?:m3u8|mp4)[^"]*)"`,
	`\{src:\s*"([^"]+)"`,
	`links=\{"[^"]+":"([^"]+)"`,
	`["'](https?://[^"']+\.m3u8[^"']*)["']`,
}

// packedPlayer resolves the many hosters that embed a JW player config, plain or packed. Some want the page
// that embeds them as Referer, others their own domain: both are tried.
func packedPlayer(ctx context.Context, c *Client, u string) (Stream, error) {
	st, err := packedPlayerWith(ctx, c, u, nil)
	if err == nil || c.Referer == Origin(u)+"/" {
		return st, err
	}
	return packedPlayerWith(ctx, c, u, map[string]string{"Referer": Origin(u) + "/"})
}

func packedPlayerWith(ctx context.Context, c *Client, u string, headers map[string]string) (Stream, error) {
	h := map[string]string{"Accept": "text/html,application/xhtml+xml"}
	for k, v := range headers {
		h[k] = v
	}
	html, final, err := c.Get(ctx, u, h)
	if err != nil {
		return Stream{}, err
	}
	return packedSource(html, final, c)
}

// packedSource finds the stream in a player page, plain or packed.
func packedSource(html, final string, c *Client) (Stream, error) {
	for _, page := range []string{html, Unpack(html)} {
		for _, p := range packedPatterns {
			if src := Find(page, p); src != "" {
				if strings.HasPrefix(src, "/") {
					src = Origin(final) + src
				}
				return Stream{URL: src, Headers: map[string]string{"Referer": final, "Origin": Origin(final), "Cookie": c.Cookies(final)}}, nil
			}
		}
	}
	return Stream{}, ErrNotFound
}

// savefiles serves its player from a POST on /dl.
func savefiles(ctx context.Context, c *Client, u string) (Stream, error) {
	id := Find(u, `/(?:e|v)/([0-9a-zA-Z]+)`)
	if id == "" {
		return packedPlayer(ctx, c, u)
	}
	ref := Origin(u) + "/"
	html, _, err := c.PostForm(ctx, ref+"dl", url.Values{"op": {"embed"}, "file_code": {id}, "auto": {"0"}, "referer": {""}},
		map[string]string{"Referer": ref, "Origin": Origin(u)})
	if err != nil {
		return Stream{}, err
	}
	return packedSource(html, ref, c)
}

// --- voe

func voe(ctx context.Context, c *Client, u string) (Stream, error) {
	html, final, err := c.Get(ctx, u, nil)
	if err != nil {
		return Stream{}, err
	}
	if strings.Contains(html, "const currentUrl") {
		if next := Find(html, `window\.location\.href\s*=\s*'([^']+)`); next != "" {
			if html, final, err = c.Get(ctx, next, nil); err != nil {
				return Stream{}, err
			}
		}
	}
	m := FindAll(html, `json">\["([^"]+)"\]</script>\s*<script\s*src="([^"]+)`)
	if len(m) == 0 {
		return Stream{}, ErrNotFound
	}
	script, _, err := c.Get(ctx, Origin(final)+m[0][1], nil)
	if err != nil {
		return Stream{}, err
	}
	luts := Find(script, `(\[(?:'\W{2}'[,\]]){1,9})`)
	if luts == "" {
		return Stream{}, ErrNotFound
	}
	data, err := voeDecode(m[0][0], luts)
	if err != nil {
		return Stream{}, err
	}
	for _, k := range []string{"file", "source"} {
		if s, ok := data[k].(string); ok && s != "" {
			return Stream{URL: s}, nil
		}
	}
	return Stream{}, ErrNotFound
}

// voeDecode: rot13, strip the junk tokens of the lookup table, base64, shift every byte by -3, reverse, base64, JSON.
func voeDecode(code, luts string) (map[string]any, error) {
	var b strings.Builder
	for _, r := range code {
		switch {
		case r >= 'A' && r <= 'Z':
			r = (r-52)%26 + 65
		case r >= 'a' && r <= 'z':
			r = (r-84)%26 + 97
		}
		b.WriteRune(r)
	}
	txt := b.String()
	for _, junk := range strings.Split(luts[2:len(luts)-2], "','") {
		txt = strings.ReplaceAll(txt, junk, "")
	}
	raw, err := base64.StdEncoding.DecodeString(txt)
	if err != nil {
		return nil, err
	}
	shifted := make([]byte, len(raw))
	for i, x := range raw {
		shifted[len(raw)-1-i] = x - 3
	}
	plain, err := base64.StdEncoding.DecodeString(string(shifted))
	if err != nil {
		return nil, err
	}
	var out map[string]any
	return out, json.Unmarshal(plain, &out)
}

// --- vidmoly

func vidmoly(ctx context.Context, c *Client, u string) (Stream, error) {
	u = strings.NewReplacer("vidmoly.to", "vidmoly.net", "vidmoly.me", "vidmoly.net").Replace(u)
	id := Find(u, `(?:embed-|/e/|/v/|/w/)([a-z0-9]+)`)
	html, final, err := c.Get(ctx, u, map[string]string{"Referer": u, "Cookie": "cf_turnstile_demo_pass_" + id + "=1"})
	if err != nil {
		return Stream{}, err
	}
	src := Find(html, `sources:\s*\[\s*\{\s*file:\s*'([^']+)'`)
	if src == "" {
		return Stream{}, ErrNotFound
	}
	return Stream{URL: src, Headers: map[string]string{"Referer": Origin(final) + "/"}}, nil
}

// --- filemoon: the playback API answers an AES-GCM encrypted source list.

func filemoon(ctx context.Context, c *Client, u string) (Stream, error) {
	api := strings.Replace(u, "/e/", "/api/videos/", 1)
	api = strings.TrimRight(api, "/") + "/embed/playback"
	body, _ := json.Marshal(filemoonFingerprint())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, bytes.NewReader(body))
	if err != nil {
		return Stream{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", u)
	req.Header.Set("Origin", Origin(u))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Stream{}, err
	}
	defer resp.Body.Close()
	var r struct {
		Playback struct {
			IV       string   `json:"iv"`
			KeyParts []string `json:"key_parts"`
			Version  any      `json:"version"`
			Payload  string   `json:"payload"`
		} `json:"playback"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Stream{}, err
	}
	p := r.Playback
	parts := p.KeyParts
	if v, _ := strconv.Atoi(fmt.Sprint(p.Version)); v > 0 && v <= len(parts) {
		parts = []string{parts[v-1], parts[len(parts)-v]}
	}
	var key []byte
	for _, part := range parts {
		key = append(key, b64url(part)...)
	}
	iv, payload := b64url(p.IV), b64url(p.Payload)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Stream{}, err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return Stream{}, err
	}
	plain, err := gcm.Open(nil, iv, payload, nil)
	if err != nil {
		return Stream{}, err
	}
	var out struct {
		Sources []struct {
			URL string `json:"url"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(plain, &out); err != nil || len(out.Sources) == 0 {
		return Stream{}, ErrNotFound
	}
	return Stream{URL: out.Sources[len(out.Sources)-1].URL}, nil
}

func b64url(s string) []byte {
	b, _ := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	return b
}

// filemoonFingerprint mimics the browser fingerprint the player posts (same shape as vStream sends).
func filemoonFingerprint() map[string]any {
	rnd := func() string {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)
	}
	now := time.Now().Unix()
	data := map[string]any{"viewer_id": rnd(), "device_id": rnd(), "confidence": 0.75, "iat": now, "exp": now + 600}
	raw, _ := json.Marshal(data)
	payload := base64.StdEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(payload))
	data["token"] = payload + "." + base64.StdEncoding.EncodeToString(sum[:])
	delete(data, "iat")
	delete(data, "exp")
	return map[string]any{"fingerprint": data}
}

// --- mixdrop

var reMixdropDomain = regexp.MustCompile(`(?i)(?:canonical|og:url)[^>]+(?:href|content)\s*=\s*["']https?://([^/"']+)`)

func mixdrop(ctx context.Context, c *Client, u string) (Stream, error) {
	headers := map[string]string{"Cookie": "hds2=1"}
	html, final, err := c.Get(ctx, u, headers)
	if err != nil {
		return Stream{}, err
	}
	if loc := Find(html, `location\s*=\s*["']([^"']+)["']`); loc != "" {
		next, err := url.Parse(final)
		if err == nil {
			if ref, err := next.Parse(loc); err == nil {
				if html, final, err = c.Get(ctx, ref.String(), headers); err != nil {
					return Stream{}, err
				}
			}
		}
	}
	src := strings.Join(strings.Fields(Find(Unpack(html), `(?:vsr|wurl|surl)[^=]*=\s*"([^"]+)`)), "")
	if src == "" {
		return Stream{}, ErrNotFound
	}
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	} else if !strings.HasPrefix(src, "http") {
		src = "https://" + src
	}
	domain := Host(final)
	if m := reMixdropDomain.FindStringSubmatch(html); m != nil {
		domain = m[1]
	}
	return Stream{URL: src, Headers: map[string]string{"Referer": "https://" + domain + "/", "Origin": "https://" + domain}}, nil
}

// --- streamtape: the page builds a get_video URL that redirects to the file.

func streamtape(ctx context.Context, c *Client, u string) (Stream, error) {
	u = strings.Replace(u, "streamtape", "tapepops", 1)
	html, final, err := c.Get(ctx, u, nil)
	if err != nil {
		return Stream{}, err
	}
	m := FindAll(html, `ById\('ideoo.+?=\s*["']([^"']+)['"].+?["']([^"']+)'\)`)
	if len(m) == 0 {
		return Stream{}, ErrNotFound
	}
	q := m[0][1]
	api := "https://tapepops.com/get_video?id=" + q + "&stream=1"
	if i := strings.Index(q, "?"); i >= 0 {
		api = "https://tapepops.com/get_video" + q[i:] + "&stream=1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return Stream{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Cookie", c.Cookies(final))
	noRedirect := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return Stream{}, err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return Stream{}, ErrNotFound
	}
	return Stream{URL: loc, Headers: map[string]string{"Referer": loc}}, nil
}

// --- veev: LZW-like obfuscated token, then the player API, then a hex/reverse encoded URL.

var reVeevItems = regexp.MustCompile(`[\.\s'](?:fc|_vvto\[[^\]]*)(?:['\]]*)?\s*[:=]\s*['"]([^'"]+)`)

func veev(ctx context.Context, c *Client, u string) (Stream, error) {
	id := Find(u, `/(?:v|embed|e|d)/([0-9a-zA-Z]+)`)
	if id == "" {
		return Stream{}, ErrNotFound
	}
	base := Origin(u)
	headers := map[string]string{"Referer": base + "/", "Origin": base}
	html, final, err := c.Get(ctx, u, headers)
	if err != nil {
		return Stream{}, err
	}
	if redirected := Find(final, `/(?:v|embed|e|d)/([0-9a-zA-Z]+)`); redirected != "" {
		id = redirected
	}
	items := reVeevItems.FindAllStringSubmatch(html, -1)
	for i := len(items) - 1; i >= 0; i-- {
		f := items[i][1]
		ch := veevDecode(f)
		if ch == f {
			continue
		}
		var r struct {
			File struct {
				Status string `json:"file_status"`
				DV     []struct {
					S string `json:"s"`
				} `json:"dv"`
			} `json:"file"`
		}
		api := fmt.Sprintf("%s/dl?op=player_api&cmd=gi&file_code=%s&ch=%s&ie=1", base, url.QueryEscape(id), url.QueryEscape(ch))
		body, _, err := c.Get(ctx, api, headers)
		if err != nil || json.Unmarshal([]byte(body), &r) != nil || r.File.Status != "OK" || len(r.File.DV) == 0 {
			continue
		}
		arrays := veevArrays(ch)
		if len(arrays) == 0 {
			continue
		}
		return Stream{URL: veevURL(veevDecode(r.File.DV[0].S), arrays[0]), Headers: map[string]string{"Referer": base + "/"}}, nil
	}
	return Stream{}, ErrNotFound
}

func veevDecode(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	lut := map[int]string{}
	n := 256
	c := string(runes[0])
	var out strings.Builder
	out.WriteString(c)
	for _, r := range runes[1:] {
		nc := string(r)
		if r >= 256 {
			if v, ok := lut[int(r)]; ok {
				nc = v
			} else {
				first, _ := utf8.DecodeRuneInString(c)
				nc = c + string(first)
			}
		}
		out.WriteString(nc)
		first, _ := utf8.DecodeRuneInString(nc)
		lut[n] = c + string(first)
		n++
		c = nc
	}
	return out.String()
}

func veevArrays(s string) [][]int {
	digit := func(r rune) int {
		if r >= '0' && r <= '9' {
			return int(r - '0')
		}
		return 0
	}
	cs := []rune(s)
	pop := func() int { v := digit(cs[0]); cs = cs[1:]; return v }
	var out [][]int
	if len(cs) == 0 {
		return out
	}
	for count := pop(); count > 0; {
		arr := make([]int, 0, count)
		for i := 0; i < count && len(cs) > 0; i++ {
			arr = append([]int{pop()}, arr...)
		}
		out = append(out, arr)
		if len(cs) == 0 {
			break
		}
		count = pop()
	}
	return out
}

func veevURL(s string, steps []int) string {
	for _, t := range steps {
		if t == 1 {
			r := []rune(s)
			for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
				r[i], r[j] = r[j], r[i]
			}
			s = string(r)
		}
		if b, err := hex.DecodeString(s); err == nil && utf8.Valid(b) {
			s = string(b)
		}
		s = strings.ReplaceAll(s, "dXRmOA==", "")
	}
	return s
}

func init() {
	Hosters = append(Hosters,
		Hoster{Name: "Sendvid", Match: []string{"sendvid"}, Resolve: sendvid},
		Hoster{Name: "Sibnet", Match: []string{"sibnet"}, Resolve: sibnet},
	)
}

func sendvid(ctx context.Context, c *Client, u string) (Stream, error) {
	html, _, err := c.Get(ctx, u, nil)
	if err != nil {
		return Stream{}, err
	}
	for _, p := range []string{`og:video" content="(.+?)"`, `<source src="([^"]+)"`} {
		if src := Find(html, p); src != "" {
			return Stream{URL: src}, nil
		}
	}
	return Stream{}, ErrNotFound
}

func sibnet(ctx context.Context, c *Client, u string) (Stream, error) {
	const main = "https://video.sibnet.ru"
	html, _, err := c.Get(ctx, u, map[string]string{"Referer": main + "/"})
	if err != nil {
		return Stream{}, err
	}
	src := Find(html, `src:.+?"([^"]+)`)
	if src == "" {
		return Stream{}, ErrNotFound
	}
	return Stream{URL: AbsURL(main, src), Headers: map[string]string{"Referer": u}}, nil
}
