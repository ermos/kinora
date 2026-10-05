package scraper

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// Port of vStream sites/frenchanimes.py (DLE site). VF and VOSTFR are separate pages, one per season;
// episodes sit in a hidden block, one line per episode: "<n>!<url>,<url>,...".
func init() {
	Sources = append(Sources, Source{ID: "frenchanimes", Name: "French Anime", DefaultURL: "https://french-anime.com/", Langs: []string{"fr"}, Find: frenchAnimesFind})
}

func frenchAnimesFind(ctx context.Context, c *Client, base string, q Query) ([]Link, error) {
	if q.Type != "tv" {
		return nil, nil
	}
	pages := map[string]string{} // lang -> page
	for _, title := range q.Names() {
		html, _, err := c.Get(ctx, base+"?do=search&mode=advanced&subaction=search&story="+url.QueryEscape(title), nil)
		if err != nil {
			return nil, err
		}
		for _, m := range FindAll(html, `mov clearfix.+?src="[^"]*" *alt="([^"]*).+?link="([^"]+).+?(?:sai">([^<]+[0-9]).+?|)Version`) {
			name := strings.TrimSpace(strings.NewReplacer("wiflix", "", "French Anime", "").Replace(m[0]))
			season := 1
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(m[2]), "Saison"))); err == nil {
				season = n
			}
			if !q.SameTitle(name) || season != q.Season {
				continue
			}
			lang := "VF"
			if strings.Contains(m[1], "vostfr") {
				lang = "VOSTFR"
			}
			if pages[lang] == "" {
				pages[lang] = m[1]
			}
		}
		if len(pages) > 0 {
			break
		}
	}
	var links []Link
	for lang, page := range pages {
		html, _, err := c.Get(ctx, page, nil)
		if err != nil {
			continue
		}
		block := Between(html, `class="eps" style="display: none">`, "</div>")
		block = strings.TrimPrefix(block, `class="eps" style="display: none">`)
		for _, line := range strings.Split(block, "\n") {
			num, urls, ok := strings.Cut(strings.TrimSpace(line), "!")
			if !ok || num != strconv.Itoa(q.Episode) {
				continue
			}
			for _, u := range strings.Split(urls, ",") {
				if u = strings.TrimSpace(u); u != "" {
					if strings.HasPrefix(u, "//") {
						u = "https:" + u
					}
					links = append(links, Link{URL: u, Lang: lang, Referer: base})
				}
			}
		}
	}
	return links, nil
}
