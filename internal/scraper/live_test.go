//go:build live

// Live checks against the real sites: go test -tags live ./internal/scraper -run Live -v
// Sites change often, run this when a source looks broken. LIVE_SOURCE=coflix limits the run to one source.
package scraper_test

import (
	"context"
	"os"
	"testing"

	. "github.com/ermos/kinora/internal/scraper"
	_ "github.com/ermos/kinora/internal/scraper/en"
	_ "github.com/ermos/kinora/internal/scraper/fr"
)

var liveQueries = map[string][]Query{
	"film":  {{Type: "movie", TMDBID: 27205, Title: "Inception", OriginalTitle: "Inception", Year: 2010}},
	"serie": {{Type: "tv", TMDBID: 1396, Title: "Breaking Bad", OriginalTitle: "Breaking Bad", Year: 2008, Season: 1, Episode: 2}},
	// Recent show, for sites that dropped older catalogs.
	"serie-recente": {{Type: "tv", TMDBID: 76479, Title: "The Boys", OriginalTitle: "The Boys", Year: 2019, Season: 2, Episode: 2}},
	// Anime as TMDB knows it: a show, absolute episodes split into seasons.
	"anime":    {{Type: "tv", TMDBID: 209867, Title: "Frieren", OriginalTitle: "葬送のフリーレン", AltTitles: []string{"Sousou no Frieren"}, Year: 2023, Season: 1, Episode: 3}},
	"anime-s2": {{Type: "tv", TMDBID: 209867, Title: "Frieren", OriginalTitle: "葬送のフリーレン", Year: 2023, Season: 2, Episode: 1}},
}

func TestLiveSources(t *testing.T) {
	only := os.Getenv("LIVE_SOURCE")
	for _, s := range Sources {
		if only != "" && s.ID != only {
			continue
		}
		for kind, qs := range liveQueries {
			if !sourceKinds[s.ID][kind] {
				continue
			}
			for _, q := range qs {
				t.Run(s.ID+"/"+kind, func(t *testing.T) {
					links, err := s.Find(context.Background(), NewClient(), s.DefaultURL, q)
					if err != nil {
						t.Fatal(err)
					}
					if len(links) == 0 {
						t.Fatal("no links")
					}
					resolved := 0
					for _, l := range links {
						h := HosterFor(l.URL)
						if h == nil {
							t.Logf("unsupported hoster: %s", l.URL)
							continue
						}
						st, err := Resolve(context.Background(), l)
						if err != nil {
							t.Logf("%s: %v", h.Name, err)
							continue
						}
						resolved++
						t.Logf("%s [%s %s] -> %.90s", h.Name, l.Lang, l.Quality, st.URL)
					}
					if resolved == 0 {
						t.Fatal("no link resolved")
					}
				})
			}
		}
	}
}

// sourceKinds says what each source serves, so the live test only asks relevant questions.
var sourceKinds = map[string]map[string]bool{
	"movix":         {"film": true, "serie": true},
	"purstream":     {"film": true, "serie": true},
	"coflix":        {"film": true, "serie": true},
	"wiflix":        {"film": true, "serie-recente": true},
	"french_stream": {"film": true, "serie": true},
	"cpasmieux":     {"film": true, "serie": true},
	"kepliz_com":    {"film": true},
	"animesama":     {"anime": true},
	"frenchanimes":  {"anime-s2": true},

	"levidia_ch": {"film": true, "serie": true},
}
