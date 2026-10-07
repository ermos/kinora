package fr

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// Streaming Integrale, not in vStream. WordPress (Toroplay theme) behind Cloudflare: searched through its HTML search
// (the REST API answers JSON that FlareSolverr wraps in a page), each player tab is an iframe page holding the embed.
func init() {
	register(scraper.Source{ID: "streaming_integrale", Name: "Streaming Integrale", DefaultURL: "https://streaming-integrale.com/", Cloudflare: true, Find: streamingIntegraleFind})
}

func streamingIntegraleFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	kind := "movie/"
	if q.Type == "tv" {
		kind = "serie/"
	}
	page := ""
	for _, title := range q.Names() {
		res, _, err := c.Get(ctx, base+"?s="+url.QueryEscape(title), nil)
		if err != nil {
			return nil, err
		}
		for _, m := range scraper.FindAll(res, `<article[^>]+TPost.+?<a href="([^"]+)".+?class="Title">([^<]+)<.+?class="Year">(\d*)<`) {
			year, _ := strconv.Atoi(m[2])
			// Titles hold entities ("Pat&rsquo;Patrouille"), except on the page FlareSolverr's browser rendered.
			if strings.Contains(m[0], "/"+kind) && q.SameTitle(html.UnescapeString(m[1])) && q.SameYear(year) {
				page = m[0]
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
	if q.Type == "tv" {
		html, _, err := c.Get(ctx, page, nil)
		if err != nil {
			return nil, err
		}
		page = scraper.Find(html, fmt.Sprintf(`href="([^"]+/episode/[^"]+-saison-%d-episode-%d/)"`, q.Season, q.Episode))
		if page == "" {
			return nil, nil
		}
	}
	html, _, err := c.Get(ctx, page, nil)
	if err != nil {
		return nil, err
	}
	var links []scraper.Link
	for _, p := range toroPlayers(html, base) {
		player, _, err := c.Get(ctx, p.URL, map[string]string{"Referer": page})
		if err != nil {
			continue
		}
		if embed := scraper.Find(player, `<iframe[^>]+src="(https?://[^"]+)"`); embed != "" {
			links = append(links, scraper.Link{URL: embed, Lang: p.Lang, Referer: base})
		}
	}
	return links, nil
}

// toroPlayers lists the player pages of a Toroplay page: tab OptN (labelled "<name> <lang>") loads ?trembed=N-1.
func toroPlayers(html, base string) []scraper.Link {
	langs := map[string]string{}
	for _, m := range scraper.FindAll(html, `data-tplayernv="Opt(\d+)"><span>[^<]*</span><span>([^<]*)<`) {
		n, _ := strconv.Atoi(m[0])
		langs[strconv.Itoa(n-1)] = normalizeLang(m[1])
	}
	var players []scraper.Link
	seen := map[string]bool{}
	for _, m := range scraper.FindAll(html, `\?trembed=(\d+)&(?:#038;)?trid=(\d+)&(?:#038;)?trtype=(\d+)`) {
		if seen[m[0]] {
			continue
		}
		seen[m[0]] = true
		players = append(players, scraper.Link{URL: fmt.Sprintf("%s?trembed=%s&trid=%s&trtype=%s", base, m[0], m[1], m[2]), Lang: langs[m[0]]})
	}
	return players
}
