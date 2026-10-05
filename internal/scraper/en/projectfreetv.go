package en

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ermos/kinora/internal/scraper"
)

// Port of Scrubs sources/projectfreetv_cyou.py (watchseries_cyou is the same site and link database). Pages are
// reached by slug; each link is an /open/link/<id>/ page whose button redirects to the hoster.
func init() {
	register(scraper.Source{ID: "projectfreetv_cyou", Name: "Project Free TV", DefaultURL: "https://freeprojecttv.cyou/", Cloudflare: true, Find: projectFreeTVFind})
}

// projectFreeTVMaxLinks bounds the redirects followed per title: pages list 20+ links, mostly on the same hosters.
const projectFreeTVMaxLinks = 8

func projectFreeTVFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	var page, html string
	for _, title := range q.Names() {
		p := fmt.Sprintf("%smovies/%s-%d/", base, slug(title), q.Year)
		if q.Type == "tv" {
			p = fmt.Sprintf("%stv-series/%s-season-%d-episode-%d/", base, slug(title), q.Season, q.Episode)
		}
		if h, _, err := c.Get(ctx, p, nil); err == nil {
			page, html = p, h
			break
		}
	}
	if page == "" {
		return nil, nil
	}
	// Rows are oldest first, and old links are mostly dead: keep the newest ones a hoster can play.
	type row struct{ page, id string }
	var rows []row
	all := scraper.FindAll(html, `<tr[^>]+class="ext_link(.+?)</tr>`)
	for i := len(all) - 1; i >= 0 && len(rows) < projectFreeTVMaxLinks; i-- {
		page, id := scraper.Find(all[i][0], `href="(/open/link/\d+/)"`), scraper.Find(all[i][0], `/open/link/(\d+)/`)
		if host := scraper.Find(all[i][0], `title="video hosted on ([^"]+)"`); id != "" && scraper.HosterFor("https://"+host+".com/") != nil {
			rows = append(rows, row{page, id})
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	// The link pages share one opener, "/open/<token>/", which redirects to the hoster.
	lp, _, err := c.Get(ctx, scraper.AbsURL(base, rows[0].page), nil)
	if err != nil {
		return nil, err
	}
	opener := scraper.Find(lp, `'(/open/[A-Za-z0-9_-]{10,}/)'`)
	if opener == "" {
		return nil, fmt.Errorf("projectfreetv: link opener not found")
	}
	urls := make([]string, len(rows))
	var wg sync.WaitGroup
	for i, r := range rows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u, err := c.Location(ctx, scraper.AbsURL(base, opener+r.id), map[string]string{"Referer": scraper.AbsURL(base, r.page)})
			if err == nil && strings.HasPrefix(u, "http") {
				urls[i] = u
			}
		}()
	}
	wg.Wait()
	var links []scraper.Link
	for _, u := range urls {
		if u != "" {
			links = append(links, scraper.Link{URL: u, Referer: page})
		}
	}
	return links, nil
}
