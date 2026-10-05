package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FetchSiteURLs returns the current URL of each source according to its sites.json (vStream format).
// A list that fails to load is skipped: its sources keep their last synced URL.
func FetchSiteURLs(ctx context.Context) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out := map[string]string{}
	var errs []error
	lists := map[string]map[string]siteEntry{}
	for _, s := range Sources {
		if s.Sites == "" {
			continue
		}
		list, ok := lists[s.Sites]
		if !ok {
			var err error
			if list, err = fetchSites(ctx, s.Sites); err != nil {
				errs = append(errs, err)
			}
			lists[s.Sites] = list
		}
		site, ok := list[s.ID]
		if !ok {
			continue
		}
		u := site.URL
		// Some sites publish their current domain on a "new address" page, fresher than sites.json.
		if site.SiteInfo != "" {
			if fresh := currentAddress(ctx, site.SiteInfo); fresh != "" {
				u = fresh
			}
		}
		if u != "" {
			out[s.ID] = strings.TrimRight(u, "/") + "/"
		}
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errs[0]
	}
	return out, nil
}

type siteEntry struct {
	URL      string `json:"url"`
	SiteInfo string `json:"site_info"`
}

func fetchSites(ctx context.Context, sitesURL string) (map[string]siteEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitesURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", sitesURL, resp.StatusCode)
	}
	var data struct {
		Sites map[string]siteEntry `json:"sites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Sites, nil
}

// currentAddress reads a "new address" page the way vStream's sites do (wiflix, french_stream...).
func currentAddress(ctx context.Context, page string) string {
	html, _, err := NewClient().Get(ctx, page, nil)
	if err != nil {
		return ""
	}
	for _, p := range []string{`location\.href\s*=\s*'(https?://[^']+)'`, `clapperboard.+?a href="(https://[^"]+)"`} {
		if u := Find(html, p); u != "" {
			return u
		}
	}
	return ""
}
