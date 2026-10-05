package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Port of vStream sites/wiflix.py (DataLife Engine site, one page per season).
func init() {
	Sources = append(Sources, Source{ID: "wiflix", Name: "Wiflix", DefaultURL: "https://flemmix.eu/", Langs: []string{"fr"}, Find: wiflixFind})
}

var reWiflixSeason = regexp.MustCompile(`(?i)^(.*?)\s*-\s*saison\s*(\d+)`)

func wiflixFind(ctx context.Context, c *Client, base string, q Query) ([]Link, error) {
	cats := []string{"1", "37"}
	pattern := `mov clearfix.+?src="[^"]*" *alt="([^"]*).+?link="([^"]+)`
	if q.Type == "tv" {
		cats = []string{"31", "35"}
		pattern = `mov clearfix.+?src="[^"]+" *alt="([^"]+).+?data-link="([^"]+)`
	}
	page := ""
	for _, title := range q.Names() {
		html, err := dleSearch(ctx, c, base, title, cats)
		if err != nil {
			return nil, err
		}
		for _, m := range FindAll(html, pattern) {
			name := strings.TrimSpace(strings.NewReplacer("flemmix", "", "wiflix", "").Replace(m[0]))
			if q.Type == "tv" {
				sm := reWiflixSeason.FindStringSubmatch(name)
				if sm == nil || !q.SameTitle(sm[1]) || sm[2] != strconv.Itoa(q.Season) {
					continue
				}
			} else if !q.SameTitle(name) {
				continue
			}
			page = m[1]
			break
		}
		if page != "" {
			break
		}
	}
	if page == "" {
		return nil, nil
	}
	html, _, err := c.Get(ctx, page, map[string]string{"Referer": base})
	if err != nil {
		return nil, err
	}
	if q.Type == "movie" {
		return loadVideoLinks(html, "", page), nil
	}
	var links []Link
	for suffix, lang := range map[string]string{"vf": "VF", "vs": "VOSTFR"} {
		block := Between(Clean(html), fmt.Sprintf(`<div class="ep%d%s"`, q.Episode, suffix), "</div>")
		links = append(links, loadVideoLinks(block, lang, page)...)
	}
	return links, nil
}

// dleSearch runs the search form of DataLife Engine sites.
func dleSearch(ctx context.Context, c *Client, base, story string, cats []string) (string, error) {
	if _, _, err := c.Get(ctx, base, nil); err != nil { // session cookie
		return "", err
	}
	form := url.Values{"do": {"search"}, "subaction": {"search"}, "story": {story}, "titleonly": {"3"}, "sortby": {"title"}, "resorder": {"asc"}}
	for _, cat := range cats {
		form.Add("catlist[]", cat)
	}
	html, _, err := c.PostForm(ctx, base+"index.php?do=search", form, map[string]string{
		"Referer": base + "index.php?do=search", "Origin": strings.TrimRight(base, "/"), "Cookie": "h_check=25; " + c.Cookies(base),
	})
	if i := strings.Index(html, "</script></form>"); i >= 0 {
		html = html[i:] // skip the "popular" sidebar printed before the results
	}
	return html, err
}

// loadVideoLinks reads the players of DLE pages: onclick="loadVideo('<embed url>')".
func loadVideoLinks(html, lang, referer string) []Link {
	var links []Link
	for _, m := range FindAll(html, `loadVideo\('([^']+)`) {
		links = append(links, Link{URL: m[0], Lang: lang, Referer: referer})
	}
	return links
}
