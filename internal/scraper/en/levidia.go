package en

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// Port of Scrubs sources/levidia_ch.py. Movies and TV shows; each link is a go.php page redirecting to a hoster.
func init() {
	register(scraper.Source{ID: "levidia_ch", Name: "Levidia", DefaultURL: "https://www.levidia.ch/", Find: levidiaFind})
}

const levidiaResult = `class="mainlink"><a href="([^"]+)"[^>]*><strong>(.+?)</strong> *\((\d{4})\)</a> *<img class="kanan" src="[^"]*/(\w+)\.png`

func levidiaFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	page, err := levidiaSearch(ctx, c, base, q)
	if err != nil || page == "" {
		return nil, err
	}
	if q.Type == "tv" {
		html, _, err := c.Get(ctx, page+"&s="+strconv.Itoa(q.Season), nil)
		if err != nil {
			return nil, err
		}
		ep := scraper.Find(html, fmt.Sprintf(`href="([^"]*-s%de%d-[^"]*)"`, q.Season, q.Episode))
		if ep == "" {
			return nil, nil
		}
		page = scraper.AbsURL(base, ep)
	}
	html, _, err := c.Get(ctx, page, nil)
	if err != nil {
		return nil, err
	}
	// go.php wants the cookie a script sets: _3chk('name','value').
	headers := map[string]string{"Referer": page}
	if m := scraper.FindAll(html, `_3chk\('([^']+)','([^']+)'\)`); len(m) > 0 {
		headers["Cookie"] = m[0][0] + "=" + m[0][1]
	}
	var links []scraper.Link
	// One <li> per link. Movie rows carry a release quality ("DVDRip"), episode rows don't.
	for _, li := range scraper.FindAll(html, `<li class="xxx0">(.+?)</li>`) {
		host := scraper.Find(li[0], `xxx1[^"]*">(?:<a[^>]*>)?<b>([^<]+)`)
		goURL := scraper.Find(li[0], `href="([^"]*go\.php[^"]*)"`)
		if goURL == "" || strings.EqualFold(host, "Wootly") { // Wootly is its own player, not a hoster
			continue
		}
		u, err := c.Location(ctx, goURL, headers)
		if err != nil {
			continue
		}
		links = append(links, scraper.Link{URL: u, Quality: strings.TrimSpace(scraper.Find(li[0], `xxx3">([^<]*)`)), Referer: page})
	}
	return links, nil
}

func levidiaSearch(ctx context.Context, c *scraper.Client, base string, q scraper.Query) (string, error) {
	kind := "movie"
	if q.Type == "tv" {
		kind = "tv"
	}
	for _, title := range q.Names() {
		html, _, err := c.PostForm(ctx, base+"search.php?q="+url.QueryEscape(title), nil, map[string]string{"Referer": base})
		if err != nil {
			return "", err
		}
		for _, m := range scraper.FindAll(html, levidiaResult) {
			year, _ := strconv.Atoi(m[2])
			if m[3] == kind && q.SameTitle(m[1]) && q.SameYear(year) {
				return m[0], nil
			}
		}
	}
	return "", nil
}
