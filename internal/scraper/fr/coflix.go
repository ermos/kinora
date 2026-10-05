package fr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// Port of vStream sites/coflix.py, adapted: the site's own search (suggest.php) and player API (apiflix)
// are broken, so titles are found through the WordPress REST search and links read from the page.
func init() {
	register(scraper.Source{ID: "coflix", Name: "Coflix", DefaultURL: "https://coflix.esq/", Find: coflixFind})
}

func coflixFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	page, err := wpSearch(ctx, c, base, q, map[string]bool{"movies": q.Type == "movie", "series": q.Type == "tv", "animes": q.Type == "tv"})
	if err != nil || page == "" {
		return nil, err
	}
	if q.Type == "tv" {
		page = fmt.Sprintf("%sepisode/%s-%dx%d/", base, path.Base(strings.TrimRight(page, "/")), q.Season, q.Episode)
	}
	html, _, err := c.Get(ctx, page, map[string]string{"Referer": base})
	if err != nil {
		return nil, err
	}
	var servers []struct {
		Embed string `json:"embed_url"`
	}
	if err := json.Unmarshal([]byte(scraper.Find(html, `cfServers\s*=\s*(\[.*?\]);`)), &servers); err != nil {
		return nil, fmt.Errorf("coflix: no player on %s", page)
	}
	token := scraper.Find(html, `cfPlayerToken\s*=\s*"([^"]+)"`)
	var links []scraper.Link
	for _, s := range servers {
		if !strings.Contains(s.Embed, "lecteurvideo") {
			links = append(links, scraper.Link{URL: s.Embed, Referer: base})
			continue
		}
		embed := s.Embed
		if token != "" {
			embed += "&t=" + url.QueryEscape(token)
		}
		player, _, err := c.Get(ctx, embed, map[string]string{"Referer": base})
		if err != nil {
			continue
		}
		for _, l := range showVideoLinks(player) {
			l.Referer = scraper.Origin(embed) + "/"
			links = append(links, l)
		}
	}
	return links, nil
}

// wpSearch finds a title through the WordPress REST search and returns its page URL. kinds lists the
// accepted post subtypes.
func wpSearch(ctx context.Context, c *scraper.Client, base string, q scraper.Query, kinds map[string]bool) (string, error) {
	for _, title := range q.Names() {
		var res []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Subtype string `json:"subtype"`
		}
		if err := c.GetJSON(ctx, base+"wp-json/wp/v2/search?per_page=20&search="+url.QueryEscape(title), &res); err != nil {
			return "", err
		}
		for _, r := range res {
			if kinds[r.Subtype] && q.SameTitle(scraper.Clean(r.Title)) {
				return r.URL, nil
			}
		}
	}
	return "", nil
}

// showVideoLinks reads the hoster list of the "lecteurvideo" family of players: onclick="showVideo('<base64 url>', ...)",
// grouped by SelLang(this, 'FR'|'VOSTFR'|...) tabs.
func showVideoLinks(html string) []scraper.Link {
	langs := scraper.FindAll(html, `SelLang\(this,\s*'([^']+)'`)
	lang := ""
	if n := len(langs); n > 0 && (n == 1 || (n == 2 && langs[1][0] == "down")) {
		lang = normalizeLang(langs[0][0])
	}
	var links []scraper.Link
	for _, m := range scraper.FindAll(html, `showVideo\('([A-Za-z0-9+/=]+)'`) {
		raw, err := base64.StdEncoding.DecodeString(m[0])
		if err != nil {
			continue
		}
		links = append(links, scraper.Link{URL: string(raw), Lang: lang})
	}
	return links
}

func normalizeLang(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "FR", "VF", "FRENCH", "TRUEFRENCH", "VFF", "VFQ":
		return "VF"
	case "VOSTFR", "VOST":
		return "VOSTFR"
	case "MULTI":
		return "MULTI"
	}
	return strings.ToUpper(strings.TrimSpace(s))
}
