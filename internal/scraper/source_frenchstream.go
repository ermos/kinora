package scraper

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Port of vStream sites/french_stream.py (DLE site with JSON player APIs: film_api.php, sx.php).
func init() {
	Sources = append(Sources, Source{ID: "french_stream", Name: "French Stream", DefaultURL: "https://fs07.lol/", Langs: []string{"fr"}, Find: frenchStreamFind})
}

var (
	reFSYear   = regexp.MustCompile(`\((\d{4})\)\s*$`)
	reFSSeason = regexp.MustCompile(`(?i)^(.*?)\s*-\s*saison\s*(\d+)`)
)

func frenchStreamFind(ctx context.Context, c *Client, base string, q Query) ([]Link, error) {
	page, err := frenchStreamSearch(ctx, c, base, q)
	if err != nil || page == "" {
		return nil, err
	}
	html, err := frenchStreamGet(ctx, c, page, base)
	if err != nil {
		return nil, err
	}
	if q.Type == "movie" {
		id := Find(html, `data-newsid="(\d+)"`)
		if id == "" {
			return nil, nil
		}
		var r struct {
			Players map[string]map[string]string `json:"players"`
		}
		if err := c.GetJSON(ctx, base+"engine/ajax/film_api.php?id="+id, &r); err != nil {
			return nil, err
		}
		var links []Link
		seen := map[string]bool{}
		for name, byLang := range r.Players {
			if name == "premium" { // fsvid, needs an account
				continue
			}
			for lang, u := range byLang {
				if u == "" || seen[u] {
					continue
				}
				seen[u] = true
				links = append(links, Link{URL: u, Lang: frenchStreamLang(lang), Referer: base})
			}
		}
		return links, nil
	}
	id := Find(html, `data-news-id="([^"]+)`)
	if id == "" {
		return nil, nil
	}
	var byLang map[string]json.RawMessage
	if err := c.GetJSON(ctx, base+"engine/ajax/sx.php?p="+id, &byLang); err != nil {
		return nil, err
	}
	var links []Link
	for lang, raw := range byLang {
		if strings.Contains(lang, "info") {
			continue
		}
		var episodes map[string]map[string]string
		if json.Unmarshal(raw, &episodes) != nil {
			continue
		}
		for name, u := range episodes[strconv.Itoa(q.Episode)] {
			if u != "" && name != "premium" && !strings.Contains(u, "uptostream") {
				links = append(links, Link{URL: u, Lang: frenchStreamLang(lang), Referer: base})
			}
		}
	}
	return links, nil
}

func frenchStreamSearch(ctx context.Context, c *Client, base string, q Query) (string, error) {
	for _, title := range q.Names() {
		form := url.Values{"query": {title}}
		headers := map[string]string{"Referer": base, "X-Requested-With": "XMLHttpRequest"}
		html, _, err := c.PostForm(ctx, base+"engine/ajax/search.php", form, headers)
		if err != nil {
			return "", err
		}
		// Anti-bot: the first answer sets a cookie through JavaScript.
		if cookie := Find(html, `document\.cookie="([^"]+)"`); cookie != "" {
			headers["Cookie"] = cookie
			if html, _, err = c.PostForm(ctx, base+"engine/ajax/search.php", form, headers); err != nil {
				return "", err
			}
		}
		for _, m := range FindAll(html, `href='([^']+).+?src='[^']+.+?title *'>([^<]+)`) {
			name := Clean(strings.ReplaceAll(m[1], "Saisn", "Saison"))
			if q.Type == "tv" {
				sm := reFSSeason.FindStringSubmatch(name)
				if sm == nil || !q.SameTitle(sm[1]) || sm[2] != strconv.Itoa(q.Season) {
					continue
				}
			} else {
				year := 0
				if y := reFSYear.FindStringSubmatch(name); y != nil {
					year, _ = strconv.Atoi(y[1])
					name = strings.TrimSpace(reFSYear.ReplaceAllString(name, ""))
				}
				if !q.SameTitle(name) || !q.SameYear(year) {
					continue
				}
			}
			return absURL(base, m[0]), nil
		}
	}
	return "", nil
}

// frenchStreamGet fetches a page, replaying the JavaScript cookie the site sets on first visit.
func frenchStreamGet(ctx context.Context, c *Client, page, base string) (string, error) {
	html, _, err := c.Get(ctx, page, map[string]string{"Referer": base})
	if err != nil {
		return "", err
	}
	if cookie := Find(html, `document\.cookie="([^"]+)"`); cookie != "" {
		html, _, err = c.Get(ctx, page, map[string]string{"Referer": base, "Cookie": cookie})
	}
	return html, err
}

func frenchStreamLang(l string) string {
	switch strings.ToLower(l) {
	case "default", "vf", "vff", "vfq":
		return "VF"
	case "vostfr", "vo":
		return "VOSTFR"
	}
	return strings.ToUpper(l)
}

func absURL(base, href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(href, "/")
}
