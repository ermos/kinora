// Package en holds the English sources, ported from Scrubs V2 (Kodi), the English counterpart of vStream.
// Importing it registers them.
package en

import (
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// SitesJSON tracks the current domains of English sites, in vStream's format. Scrubs keeps its domains
// in code, so this list lives in this repo (sites.json next to this file): editing it on main updates
// every instance at its next sync, without a release.
const SitesJSON = "https://raw.githubusercontent.com/ermos/kinora/main/internal/scraper/en/sites.json"

func init() {
	scraper.Languages = append(scraper.Languages, scraper.Language{Code: "en", Label: "English", TMDB: "en-US", AudioRank: audioRank})
}

func register(s scraper.Source) {
	s.Langs, s.Sites = []string{"en"}, SitesJSON
	scraper.Sources = append(scraper.Sources, s)
}

// audioRank: English sites only carry the original audio, so tags rarely exist. Subtitled versions
// (anime, foreign films) play after.
func audioRank(tag string) int {
	if strings.Contains(strings.ToUpper(tag), "SUB") {
		return 0
	}
	return 1
}
