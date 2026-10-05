package scraper

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// Port of vStream sites/kepliz_com.py (movies only). The real site sits under a random path that changes,
// read from the landing page.
func init() {
	Sources = append(Sources, Source{ID: "kepliz_com", Name: "Kepliz", DefaultURL: "https://kambad.com/", Langs: []string{"fr"}, Find: keplizFind})
}

func keplizFind(ctx context.Context, c *Client, base string, q Query) ([]Link, error) {
	if q.Type != "movie" {
		return nil, nil
	}
	landing, _, err := c.Get(ctx, base, nil)
	if err != nil {
		return nil, err
	}
	prefix := strings.Trim(Find(landing, `<a.+?href="(/*[0-9a-zA-Z]+)"`), "/")
	if prefix == "" {
		return nil, nil
	}
	main := base + prefix + "/"
	searchURL := main + "home/" + strings.Split(Host(base), ".")[0]
	host := strings.TrimRight(base, "/")

	page := ""
	for _, title := range q.Names() {
		words := Normalize(title)
		if len([]rune(title)) > 20 { // the site's search breaks on long queries
			title = string([]rune(title)[:20])
		}
		html, _, err := c.PostForm(ctx, searchURL, url.Values{"searchword": {strings.ReplaceAll(title, " ", "%")}, "Referer": {searchURL}}, map[string]string{"Cookie": "g=true", "Referer": searchURL})
		if err != nil {
			return nil, err
		}
		for _, m := range FindAll(html, `<a class="film-card" href="([^"]+)">\s*<img class="film-card-img" src="[^"]+" alt="([^"]+)".+?"trend-card-date">([^<>]+)<`) {
			year, _ := strconv.Atoi(strings.TrimSpace(m[2]))
			if (q.SameTitle(m[1]) || Normalize(m[1]) == words) && q.SameYear(year) {
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
	frame := Find(html, `<iframe.+?src="([^"]+)`)
	if frame == "" {
		return nil, nil
	}
	player, _, err := c.Get(ctx, absURL(base, frame), map[string]string{"Referer": page})
	if err != nil {
		return nil, err
	}
	if next := Find(player, `document\.location\.href="([^"]+)`); next != "" {
		if player, _, err = c.Get(ctx, absURL(base, next), map[string]string{"Referer": page}); err != nil {
			return nil, err
		}
	}
	var links []Link
	for _, m := range FindAll(player, `file: *"(.+?)"`) {
		links = append(links, Link{URL: m[0], Lang: "VF", Headers: map[string]string{"Referer": Origin(m[0]) + "/"}})
	}
	return links, nil
}
