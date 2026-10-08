package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// FlareSolverr (github.com/FlareSolverr/FlareSolverr) passes Cloudflare challenges in a real browser. Once an admin
// sets its URL, FindLinks queries the sources marked Cloudflare, and their challenged requests go through it.

// clearances keeps, per host, the cookies and user agent of the last solved challenge. Requests reuse them and only
// go back to the browser when Cloudflare challenges again.
var clearances sync.Map // host -> clearance

type clearance struct {
	cookies   []*http.Cookie
	userAgent string
}

func isChallenge(resp *http.Response) bool {
	return resp.Header.Get("Cf-Mitigated") == "challenge" ||
		(resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable) &&
			strings.HasPrefix(resp.Header.Get("Server"), "cloudflare")
}

type solverResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Solution struct {
		URL       string `json:"url"`
		Status    int    `json:"status"`
		Response  string `json:"response"`
		UserAgent string `json:"userAgent"`
		Cookies   []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	} `json:"solution"`
}

// solve fetches rawURL through FlareSolverr, remembers the clearance for its host, and returns the page and final URL.
// FlareSolverr can't forward headers: requests needing a Referer or a custom header lose them.
func solve(ctx context.Context, solver, method, rawURL, form string) (string, string, error) {
	cmd := map[string]any{"cmd": "request.get", "url": rawURL, "maxTimeout": 60000}
	if method == http.MethodPost {
		cmd["cmd"], cmd["postData"] = "request.post", form
	}
	body, _ := json.Marshal(cmd)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(solver, "/")+"/v1", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("flaresolverr: %w", err)
	}
	defer resp.Body.Close()
	var out solverResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("flaresolverr: %w", err)
	}
	if out.Status != "ok" {
		return "", "", fmt.Errorf("flaresolverr: %s", out.Message)
	}
	sol := out.Solution
	cl := clearance{userAgent: sol.UserAgent}
	for _, c := range sol.Cookies {
		cl.cookies = append(cl.cookies, &http.Cookie{Name: c.Name, Value: c.Value}) //nolint:gosec // G124: sent upstream, never set on a client
	}
	clearances.Store(Host(rawURL), cl)
	if sol.Status >= 400 {
		return "", "", fmt.Errorf("%s %s: status %d (flaresolverr)", method, rawURL, sol.Status)
	}
	return sol.Response, sol.URL, nil
}

// PingFlareSolverr checks that u answers like a FlareSolverr server.
func PingFlareSolverr(ctx context.Context, u string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if p, err := url.Parse(u); err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
		return fmt.Errorf("invalid URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Msg string `json:"msg"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || !strings.Contains(out.Msg, "FlareSolverr") {
		return fmt.Errorf("not a FlareSolverr server")
	}
	return nil
}
