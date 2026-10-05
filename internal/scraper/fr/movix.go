package fr

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// Port of vStream sites/movix.py.
func init() {
	register(scraper.Source{ID: "movix", Name: "Movix", DefaultURL: "https://movix.zip/", Find: movixFind})
}

const movixResult = `overflow-hidden">.+?href="([^"]+)".+?data-src="([^"]+)" alt="([^"]+)".+?<span>([^<]+)</span> *<!--\[if ENDBLOCK\]><!\[endif\]--> *</div>.+?ml-auto">([^<>]+)`

func movixFind(ctx context.Context, c *scraper.Client, base string, q scraper.Query) ([]scraper.Link, error) {
	page, err := movixSearch(ctx, c, base, q)
	if err != nil || page == "" {
		return nil, err
	}
	if q.Type == "tv" {
		page = fmt.Sprintf("%sepisode/%s/%d-%d", base, path.Base(page), q.Season, q.Episode)
	}
	html, _, err := c.Get(ctx, page, nil)
	if err != nil {
		return nil, err
	}
	var links []scraper.Link
	for _, m := range scraper.FindAll(scraper.Between(html, "currentVersionVideos() {", "];"), `version":"([^"]+)".+?link":"([^"]+)"`) {
		links = append(links, scraper.Link{Lang: m[0], URL: m[1]})
	}
	return links, nil
}

// movixSearch returns the page URL of the title, trying the localized then the original title.
func movixSearch(ctx context.Context, c *scraper.Client, base string, q scraper.Query) (string, error) {
	for _, title := range q.Names() {
		html, _, err := c.Get(ctx, base+"search/"+url.PathEscape(title), nil)
		if err != nil {
			return "", err
		}
		for _, m := range scraper.FindAll(html, movixResult) {
			href, name, kind := m[0], m[2], strings.TrimSpace(m[4])
			year, _ := strconv.Atoi(strings.TrimSpace(m[3]))
			isMovie := kind == "Film" || kind == "Movie"
			if isMovie != (q.Type == "movie") || !q.SameTitle(name) || !q.SameYear(year) {
				continue
			}
			return href, nil
		}
	}
	return "", nil
}
