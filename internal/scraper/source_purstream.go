package scraper

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Port of vStream sites/purstream.py. Purstream exposes a JSON API and serves HLS directly.
func init() {
	Sources = append(Sources, Source{ID: "purstream", Name: "Purstream", DefaultURL: "https://purstream.ad/", Langs: []string{"fr"}, Find: purstreamFind})
}

type purstreamResp[T any] struct {
	Type string `json:"type"`
	Data struct {
		Items T `json:"items"`
	} `json:"data"`
}

func purstreamFind(ctx context.Context, c *Client, base string, q Query) ([]Link, error) {
	// ponytail: the API host is derived from the site host (purstream.ad -> api.purstream.ad), like sites.json's url_api today.
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	api := fmt.Sprintf("%s://api.%s/api/v1/", u.Scheme, u.Host)

	id, err := purstreamSearch(ctx, c, api, q)
	if err != nil || id == 0 {
		return nil, err
	}
	stream := fmt.Sprintf("%sstream/%d", api, id)
	if q.Type == "tv" {
		stream += fmt.Sprintf("/episode?season=%d&episode=%d", q.Season, q.Episode)
	}
	var resp purstreamResp[struct {
		Sources []struct {
			URL  string `json:"stream_url"`
			Name string `json:"source_name"`
		} `json:"sources"`
	}]
	if err := c.GetJSON(ctx, stream, &resp); err != nil {
		return nil, err
	}
	var links []Link
	for _, s := range resp.Data.Items.Sources {
		// source_name looks like "pulse | 1080p | MULTI"
		parts := strings.Split(s.Name, "|")
		l := Link{URL: s.URL, Headers: map[string]string{"Referer": base}}
		if len(parts) == 3 {
			l.Quality, l.Lang = strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
		}
		links = append(links, l)
	}
	return links, nil
}

func purstreamSearch(ctx context.Context, c *Client, api string, q Query) (int, error) {
	for _, title := range q.Names() {
		var resp purstreamResp[struct {
			Movies struct {
				Items []struct {
					ID          int    `json:"id"`
					Title       string `json:"title"`
					Type        string `json:"type"`
					ReleaseDate string `json:"release_date"`
				} `json:"items"`
			} `json:"movies"`
		}]
		if err := c.GetJSON(ctx, api+"search-bar/search/"+url.PathEscape(title)+"?types="+q.Type, &resp); err != nil {
			return 0, err
		}
		for _, m := range resp.Data.Items.Movies.Items {
			year := 0
			if len(m.ReleaseDate) >= 4 {
				year, _ = strconv.Atoi(m.ReleaseDate[:4])
			}
			if m.Type != q.Type || !q.SameTitle(m.Title) || !q.SameYear(year) {
				continue
			}
			// The sheet carries the TMDB id: exact match when both sides know it.
			var sheet purstreamResp[struct {
				TMDBID int `json:"tmdbId"`
			}]
			if err := c.GetJSON(ctx, fmt.Sprintf("%smedia/%d/sheet", api, m.ID), &sheet); err == nil &&
				sheet.Data.Items.TMDBID != 0 && q.TMDBID != 0 && sheet.Data.Items.TMDBID != q.TMDBID {
				continue
			}
			return m.ID, nil
		}
	}
	return 0, nil
}
