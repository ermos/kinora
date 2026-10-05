package fr

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// Port of vStream sites/cpasmieux.py: search, then for shows the season page, then the episode page.
func init() {
	register(scraper.Source{ID: "cpasmieux", Name: "Cpasmieux", DefaultURL: "https://www.cpasmieux.is/", Find: cpasmieuxFind})
}

var reCpasSeason = regexp.MustCompile(`(?i)saison\s*(\d+)\s*$`)

func cpasmieuxFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	page := ""
	for _, title := range q.Names() {
		html, _, err := c.Get(ctx, base+"search/"+url.PathEscape(title), nil)
		if err != nil {
			return nil, err
		}
		for _, m := range scraper.FindAll(html, `link" href="([^"]+).+?<img src="([^"]+).+?alt="([^"]+)`) {
			isShow := strings.Contains(m[1], "saison")
			if isShow == (q.Type == "movie") || !q.SameTitle(m[2]) {
				continue
			}
			page = scraper.AbsURL(base, m[0])
			break
		}
		if page != "" {
			break
		}
	}
	if page == "" {
		return nil, nil
	}
	html, _, err := c.Get(ctx, page, nil)
	if err != nil {
		return nil, err
	}
	if q.Type == "tv" {
		if html, err = cpasmieuxEpisode(ctx, c, base, html, q); err != nil || html == "" {
			return nil, err
		}
	}
	var links []scraper.Link
	for _, m := range scraper.FindAll(html, `data-url="([^"]+)".+?"serv">([^<]+)`) {
		links = append(links, scraper.Link{URL: m[0], Referer: base})
	}
	return links, nil
}

// cpasmieuxEpisode walks a show page to the episode page: seasons block, then the episodes block.
func cpasmieuxEpisode(ctx context.Context, c *scraper.Client, base, html string, q scraper.Query) (string, error) {
	seasonPage := ""
	for _, m := range scraper.FindAll(scraper.Between(scraper.Clean(html), `class="seasons"`, "smart-text-s"), `<a href="([^"]+).+?img src="[^"]+" alt="([^"]+)`) {
		if sm := reCpasSeason.FindStringSubmatch(m[1]); sm != nil && sm[1] == strconv.Itoa(q.Season) {
			seasonPage = scraper.AbsURL(base, m[0])
			break
		}
	}
	if seasonPage == "" {
		return "", nil
	}
	season, _, err := c.Get(ctx, seasonPage, nil)
	if err != nil {
		return "", err
	}
	clean := scraper.Clean(season)
	start := strings.Index(clean, `class="seasons"`)
	if start < 0 {
		return "", nil
	}
	block := clean[start:]
	if end := strings.Index(block[1:], `class="seasons"`); end >= 0 {
		block = block[:end+1]
	}
	for _, m := range scraper.FindAll(block, `href="([^"]+).+?span>(\d+)`) {
		if m[1] == strconv.Itoa(q.Episode) {
			ep, _, err := c.Get(ctx, scraper.AbsURL(base, m[0]), nil)
			return ep, err
		}
	}
	return "", nil
}
