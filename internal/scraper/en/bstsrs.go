package en

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// Port of Scrubs sources/bstsrs_one.py. TV shows only; episode pages are reached by slug, no search.
func init() {
	register(scraper.Source{ID: "bstsrs_one", Name: "Bstsrs", DefaultURL: "https://bstsrs.in/", Cloudflare: true, Find: bstsrsFind})
}

func bstsrsFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	if q.Type != "tv" {
		return nil, nil
	}
	sepi := fmt.Sprintf("s%02de%02d/season/%d/episode/%d", q.Season, q.Episode, q.Season, q.Episode)
	for _, title := range q.Names() {
		// The slug carries the year when several shows share a title ("doctor-who-2005").
		for _, s := range []string{slug(fmt.Sprintf("%s %d", title, q.Year)), slug(title)} {
			page := base + "show/" + s + "-" + sepi
			html, _, err := c.Get(ctx, page, map[string]string{"Referer": base})
			if err != nil || !bstsrsSameShow(q, html) {
				continue // 404: wrong slug
			}
			var links []scraper.Link
			for _, enc := range scraper.FindAll(html, `dbneg\('([0-9a-f-]+)'\)`) {
				if u := dbneg(enc[0]); strings.HasPrefix(u, "http") {
					links = append(links, scraper.Link{URL: u, Referer: page})
				}
			}
			return links, nil
		}
	}
	return nil, nil
}

// bstsrsSameShow checks the keywords meta, "Breaking Bad 2008,S01E01,...", against the query.
func bstsrsSameShow(q scraper.Query, html string) bool {
	kw := scraper.Find(html, `name="keywords" content="([^,"]+)`)
	i := strings.LastIndex(kw, " ")
	if i < 0 {
		return false
	}
	year, _ := strconv.Atoi(kw[i+1:])
	return q.SameTitle(kw[:i]) && q.SameYear(year)
}

// dbneg decodes the site's link obfuscation: dash-separated hex numbers, each a char code plus a key.
// Links always start with "h", which gives the key.
func dbneg(enc string) string {
	parts := strings.Split(enc, "-")
	first, err := strconv.ParseInt(parts[0], 16, 64)
	if err != nil {
		return ""
	}
	key := first - 'h'
	var b strings.Builder
	for _, p := range parts {
		n, err := strconv.ParseInt(p, 16, 64)
		if err != nil {
			return ""
		}
		b.WriteRune(rune(n - key))
	}
	return b.String()
}
