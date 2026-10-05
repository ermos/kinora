package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SitesJSON is vStream's list of current site domains, updated by their team several times a week.
const SitesJSON = "https://raw.githubusercontent.com/Kodi-vStream/venom-xbmc-addons/Beta/plugin.video.vstream/resources/sites.json"

// FetchSiteURLs returns the current URL of each ported source according to vStream's sites.json.
func FetchSiteURLs(ctx context.Context) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SitesJSON, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sites.json: status %d", resp.StatusCode)
	}
	var data struct {
		Sites map[string]struct {
			URL      string `json:"url"`
			SiteInfo string `json:"site_info"`
		} `json:"sites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, s := range Sources {
		site, ok := data.Sites[s.ID]
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
	return out, nil
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
