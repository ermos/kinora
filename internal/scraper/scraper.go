// Package scraper is the Go port of vStream's sites (sources) and hosters.
//
// A Source finds links for a title. A link usually points to an embed page on a
// hoster, which a Hoster resolves to a playable stream (HLS or file) plus the
// headers the CDN expects.
package scraper

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Query struct {
	Lang          string // instance language (ISO 639-1)
	Type          string // "movie" or "tv"
	TMDBID        int
	Title         string
	OriginalTitle string
	// AltTitles are TMDB alternative titles (romaji, English...), tried when the main titles find nothing.
	AltTitles []string
	Year      int
	Season    int
	Episode   int
}

// Names lists the titles to search with, most likely first.
func (q Query) Names() []string {
	return uniq(append([]string{q.Title, q.OriginalTitle}, q.AltTitles...)...)
}

type Link struct {
	Source  string            `json:"source"`
	Hoster  string            `json:"hoster"`
	Lang    string            `json:"lang"`
	Quality string            `json:"quality"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	// Referer is the page the embed was found on; hosters often refuse requests without it.
	Referer string `json:"referer,omitempty"`

	// Set by probe.
	Tier       int     `json:"tier,omitempty"`
	Dead       bool    `json:"dead,omitempty"`
	Stream     *Stream `json:"stream,omitempty"`
	ResolvedAt int64   `json:"resolvedAt,omitempty"`
}

type Stream struct {
	URL     string            `json:"u"`
	Headers map[string]string `json:"h,omitempty"`
}

type Source struct {
	ID         string
	Name       string
	DefaultURL string   // used until the first sites.json sync
	Langs      []string // languages of the site's audience (ISO 639-1)
	// Find returns the links for q. base is the site URL (synced from vStream's sites.json or overridden by an admin).
	Find func(ctx context.Context, c *Client, base string, q Query) ([]Link, error)
}

type Hoster struct {
	Name string
	// Match lists host substrings, like vStream's checkHoster.
	Match   []string
	Resolve func(ctx context.Context, c *Client, url string) (Stream, error)
}

var (
	Sources []Source
	Hosters []Hoster
)

var ErrNotFound = errors.New("stream not found")

// SiteURL gives the current URL of a source, synced from vStream's sites.json ("" before the first sync).
type SiteURL func(id string) string

// FindLinks queries the sources of the language concurrently, keeps the links a hoster can play, then probes them
// (quality, dead links) and sorts them best first.
func FindLinks(ctx context.Context, siteURL SiteURL, q Query, lang Language) []Link {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	var (
		mu  sync.Mutex
		all []Link
		wg  sync.WaitGroup
	)
	for _, s := range Sources {
		if !s.serves(lang.Code) {
			continue
		}
		base := siteURL(s.ID)
		if base == "" {
			base = s.DefaultURL
		}
		if base == "" {
			continue
		}
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			links, err := s.Find(ctx, NewClient(), strings.TrimRight(base, "/")+"/", q)
			if err != nil {
				slog.Warn("source failed", "source", s.ID, "err", err)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, l := range links {
				h := HosterFor(l.URL)
				if h == nil {
					slog.Debug("no hoster", "source", s.ID, "url", l.URL)
					continue
				}
				l.Source, l.Hoster = s.Name, h.Name
				all = append(all, l)
			}
		}(s)
	}
	wg.Wait()
	probe(ctx, all)
	SortLinks(all, lang.AudioRank)
	return all
}

// streamFresh is how long a stream resolved by probe is reused; hoster URLs are signed for a few hours.
const streamFresh = 30 * time.Minute

func Resolve(ctx context.Context, l Link) (Stream, error) {
	if l.Stream != nil && time.Since(time.Unix(l.ResolvedAt, 0)) < streamFresh {
		return *l.Stream, nil
	}
	h := HosterFor(l.URL)
	if h == nil {
		return Stream{}, ErrNotFound
	}
	c := NewClient()
	c.Referer = l.Referer
	st, err := h.Resolve(ctx, c, l.URL)
	if err != nil {
		return Stream{}, err
	}
	if st.Headers == nil {
		st.Headers = map[string]string{}
	}
	for k, v := range l.Headers {
		if _, ok := st.Headers[k]; !ok {
			st.Headers[k] = v
		}
	}
	return st, nil
}

func HosterFor(rawURL string) *Hoster {
	if isMedia(rawURL) {
		return &direct
	}
	host := Host(rawURL)
	for i := range Hosters {
		for _, m := range Hosters[i].Match {
			if strings.Contains(host, m) {
				return &Hosters[i]
			}
		}
	}
	if reEmbedPath.MatchString(rawURL) {
		return &sniffed
	}
	return nil
}

var reEmbedPath = regexp.MustCompile(`/(?:e|v|embed)/[A-Za-z0-9]+|/embed-[A-Za-z0-9]+`)

// sniffed handles unknown embed domains, like vStream does: voe clones change domain every week, and many
// unknown hosters run the same JW player as filelions & co.
var sniffed = Hoster{Name: "Lecteur", Resolve: func(ctx context.Context, c *Client, u string) (Stream, error) {
	html, _, err := c.Get(ctx, u, nil)
	if err != nil {
		return Stream{}, err
	}
	if strings.Contains(html, "VOE") || strings.Contains(html, "voe.sx") || strings.Contains(html, "const currentUrl") {
		if st, err := voe(ctx, c, u); err == nil {
			return st, nil
		}
	}
	return packedPlayer(ctx, c, u)
}}

func isMedia(u string) bool {
	path := strings.ToLower(strings.SplitN(u, "?", 2)[0])
	for _, ext := range []string{".m3u8", ".mp4", ".mkv", ".webm"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

var direct = Hoster{Name: "Direct", Resolve: func(_ context.Context, _ *Client, u string) (Stream, error) {
	return Stream{URL: u}, nil
}}

var accents = strings.NewReplacer("é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a", "î", "i", "ï", "i",
	"ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c", "œ", "oe", "æ", "ae")

// Normalize lowercases, strips accents and keeps only letters and digits, to compare titles across sites.
func Normalize(s string) string {
	s = accents.Replace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SameTitle reports whether a site title matches the query (localized, original or alternative title).
func (q Query) SameTitle(title string) bool {
	n := Normalize(title)
	if n == "" {
		return false
	}
	for _, name := range q.Names() {
		if n == Normalize(name) {
			return true
		}
	}
	return false
}

// SameYear tolerates one year of difference (release dates differ between countries); unknown years match.
func (q Query) SameYear(year int) bool {
	return q.Year == 0 || year == 0 || (year-q.Year <= 1 && q.Year-year <= 1)
}
