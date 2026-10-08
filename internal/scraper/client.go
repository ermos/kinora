package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ermos/kinora/internal/safehttp"
)

const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"

// Client is a short-lived HTTP client with its own cookie jar, one per scrape or resolve.
type Client struct {
	http *http.Client
	jar  http.CookieJar
	// Referer is sent when a request sets none: the page an embed was found on, which many hosters check.
	Referer string
	// Solver is the FlareSolverr URL that requests challenged by Cloudflare go through ("" gives up on them).
	Solver string
	// AcceptLanguage is sent on every request: sources get the instance language, hosters French by default.
	AcceptLanguage string
}

// transport only reaches public addresses: the URLs scraped come from third-party pages. Tests swap it to reach
// their local servers.
var transport http.RoundTripper = safehttp.Transport()

func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{jar: jar, AcceptLanguage: "fr-FR,fr;q=0.9", http: &http.Client{Jar: jar, Timeout: 20 * time.Second, Transport: transport}}
}

// Get fetches a page and returns its body and the final URL after redirects.
func (c *Client) Get(ctx context.Context, rawURL string, headers map[string]string) (string, string, error) {
	return c.do(ctx, http.MethodGet, rawURL, "", headers)
}

// PostForm posts an urlencoded form, like the search forms of DLE and WordPress sites.
func (c *Client) PostForm(ctx context.Context, rawURL string, form url.Values, headers map[string]string) (string, string, error) {
	h := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	for k, v := range headers {
		h[k] = v
	}
	return c.do(ctx, http.MethodPost, rawURL, form.Encode(), h)
}

func (c *Client) do(ctx context.Context, method, rawURL, body string, headers map[string]string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept-Language", c.AcceptLanguage)
	if c.Referer != "" {
		req.Header.Set("Referer", c.Referer)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.useClearance(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if c.Solver != "" && isChallenge(resp) {
		return solve(ctx, c.Solver, method, rawURL, body)
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("%s %s: status %d", method, rawURL, resp.StatusCode)
	}
	return string(page), resp.Request.URL.String(), nil
}

// Location returns where rawURL redirects to without following it, for "go.php" style link pages.
func (c *Client) Location(ctx context.Context, rawURL string, headers map[string]string) (string, error) {
	nc := *c.http
	nc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.useClearance(req)
	resp, err := nc.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if c.Solver != "" && isChallenge(resp) {
		// The browser follows the redirect: where it lands is the answer.
		_, final, err := solve(ctx, c.Solver, http.MethodGet, rawURL, "")
		return final, err
	}
	loc, err := resp.Location()
	if err != nil {
		return "", fmt.Errorf("GET %s: no redirect (status %d)", rawURL, resp.StatusCode)
	}
	return loc.String(), nil
}

// useClearance sends the cookies and user agent of a challenge solved for this host: the clearance only holds
// with the browser's user agent.
func (c *Client) useClearance(req *http.Request) {
	if c.Solver == "" {
		return
	}
	if v, ok := clearances.Load(Host(req.URL.String())); ok {
		cl := v.(clearance)
		req.Header.Set("User-Agent", cl.userAgent)
		c.jar.SetCookies(req.URL, cl.cookies)
	}
}

func (c *Client) GetJSON(ctx context.Context, rawURL string, out any) error {
	body, _, err := c.Get(ctx, rawURL, map[string]string{"Accept": "application/json"})
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(body), out)
}

// Cookies returns the Cookie header the jar would send to rawURL.
func (c *Client) Cookies(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := make([]string, 0)
	for _, ck := range c.jar.Cookies(u) {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	return strings.Join(parts, "; ")
}

var cleaner = strings.NewReplacer(
	"\r", "", "\n", "", "\t", "", `\/`, "/", "&amp;", "&", "&#039;", "'", "&quot;", `"`,
	"&#8211;", "-", "&#8212;", "-", "&eacute;", "é", "&egrave;", "è", "&hellip;", "...", "&gt;", ">", "&lt;", "<",
)

// Clean mirrors vStream's cParser preprocessing: patterns are written against single-line, unescaped HTML.
func Clean(html string) string { return cleaner.Replace(html) }

var reCache sync.Map

func re(pattern string) *regexp.Regexp {
	if r, ok := reCache.Load(pattern); ok {
		return r.(*regexp.Regexp)
	}
	r := regexp.MustCompile("(?i)" + pattern)
	reCache.Store(pattern, r)
	return r
}

// FindAll runs a vStream-style pattern on cleaned HTML and returns the capture groups of every match.
func FindAll(html, pattern string) [][]string {
	ms := re(pattern).FindAllStringSubmatch(Clean(html), -1)
	out := make([][]string, len(ms))
	for i, m := range ms {
		out[i] = m[1:]
	}
	return out
}

// Find returns the first capture group of the first match, or "".
func Find(html, pattern string) string {
	m := re(pattern).FindStringSubmatch(Clean(html))
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// Between returns the substring from start up to end, like cParser.abParse; end is searched after start.
func Between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	s = s[i:]
	if j := strings.Index(s[len(start):], end); j >= 0 {
		return s[:len(start)+j]
	}
	return s
}

func Host(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func Origin(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// AbsURL resolves a site-relative href against base.
func AbsURL(base, href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(href, "/")
}
