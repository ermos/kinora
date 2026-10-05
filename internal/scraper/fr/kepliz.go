package fr

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// Port of vStream sites/kepliz_com.py (movies only). The real site sits under a random path that changes,
// read from the landing page.
func init() {
	register(scraper.Source{ID: "kepliz_com", Name: "Kepliz", DefaultURL: "https://kambad.com/", Find: keplizFind})
}

func keplizFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	if q.Type != "movie" {
		return nil, nil
	}
	landing, _, err := c.Get(ctx, base, nil)
	if err != nil {
		return nil, err
	}
	prefix := strings.Trim(scraper.Find(landing, `<a.+?href="(/*[0-9a-zA-Z]+)"`), "/")
	if prefix == "" {
		return nil, nil
	}
	main := base + prefix + "/"
	searchURL := main + "home/" + strings.Split(scraper.Host(base), ".")[0]
	host := strings.TrimRight(base, "/")

	page := ""
	for _, title := range q.Names() {
		words := scraper.Normalize(title)
		if len([]rune(title)) > 20 { // the site's search breaks on long queries
			title = string([]rune(title)[:20])
		}
		html, _, err := c.PostForm(ctx, searchURL, url.Values{"searchword": {strings.ReplaceAll(title, " ", "%")}, "Referer": {searchURL}}, map[string]string{"Cookie": "g=true", "Referer": searchURL})
		if err != nil {
			return nil, err
		}
		for _, m := range scraper.FindAll(html, `<a class="film-card" href="([^"]+)">\s*<img class="film-card-img" src="[^"]+" alt="([^"]+)".+?"trend-card-date">([^<>]+)<`) {
			year, _ := strconv.Atoi(strings.TrimSpace(m[2]))
			if (q.SameTitle(m[1]) || scraper.Normalize(m[1]) == words) && q.SameYear(year) {
				page = host + m[0]
				break
			}
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
	frame := scraper.Find(html, `<iframe.+?src="([^"]+)`)
	if frame == "" {
		return nil, nil
	}
	player, _, err := c.Get(ctx, scraper.AbsURL(base, frame), map[string]string{"Referer": page})
	if err != nil {
		return nil, err
	}
	if next := scraper.Find(player, `document\.location\.href="([^"]+)`); next != "" {
		if player, _, err = c.Get(ctx, scraper.AbsURL(base, next), map[string]string{"Referer": page}); err != nil {
			return nil, err
		}
	}
	var links []scraper.Link
	for _, m := range scraper.FindAll(player, `file: *"(.+?)"`) {
		links = append(links, scraper.Link{URL: m[0], Lang: "VF", Headers: map[string]string{"Referer": scraper.Origin(m[0]) + "/"}})
	}
	return links, nil
}
