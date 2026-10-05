package scraper

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Port of vStream sites/animesama.py. Seasons are "Saison N" panels; each language has an episodes.js
// holding one array of embed URLs per player, indexed by episode.
func init() {
	Sources = append(Sources, Source{ID: "animesama", Name: "Anime-Sama", DefaultURL: "https://anime-sama.to/", Langs: []string{"fr"}, Find: animeSamaFind})
}

func animeSamaFind(ctx context.Context, c *Client, base string, q Query) ([]Link, error) {
	if q.Type != "tv" { // ponytail: anime films are panels of the show page, matching them to TMDB movies comes later
		return nil, nil
	}
	page := ""
	for _, title := range q.Names() {
		html, _, err := c.Get(ctx, base+"catalogue/?search="+url.QueryEscape(title), nil)
		if err != nil {
			return nil, err
		}
		for _, m := range FindAll(html, `card-base"> *<a href="([^"]+).+?src="[^"]+" *alt="([^"]+)`) {
			if q.SameTitle(m[1]) {
				page = strings.TrimRight(absURL(base, m[0]), "/")
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
	seasonPath := ""
	for _, m := range FindAll(html, `panneauAnime\("([^"]+)", "([^"]+)`) {
		if strings.EqualFold(strings.TrimSpace(m[0]), fmt.Sprintf("Saison %d", q.Season)) {
			seasonPath = strings.Split(m[1], "/")[0]
			break
		}
	}
	if seasonPath == "" {
		return nil, nil
	}
	var links []Link
	for _, lang := range []string{"vostfr", "vf"} {
		seasonURL := page + "/" + seasonPath + "/" + lang
		season, _, err := c.Get(ctx, seasonURL, nil)
		if err != nil {
			continue // no VF for many animes
		}
		js := Find(season, `<script type="text/javascript" src=['"]([^'"]+)['"] *defer`)
		if js == "" {
			continue
		}
		data, _, err := c.Get(ctx, seasonURL+"/"+js, map[string]string{"Referer": seasonURL})
		if err != nil {
			continue
		}
		for _, arr := range FindAll(data, `var eps\d+ = \[(.+?)\];`) {
			urls := FindAll(arr[0], `'([^']*)'`)
			if q.Episode-1 < len(urls) && urls[q.Episode-1][0] != "" {
				links = append(links, Link{URL: urls[q.Episode-1][0], Lang: strings.ToUpper(lang), Referer: base})
			}
		}
	}
	return links, nil
}
