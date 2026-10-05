// Package fr holds the French sources, ported from vStream (Kodi). Importing it registers them.
package fr

import (
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// SitesJSON is vStream's list of current site domains, updated by their team several times a week.
const SitesJSON = "https://raw.githubusercontent.com/Kodi-vStream/venom-xbmc-addons/Beta/plugin.video.vstream/resources/sites.json"

func init() {
	scraper.Languages = append(scraper.Languages, scraper.Language{Code: "fr", Label: "Français", TMDB: "fr-FR", AudioRank: audioRank})
}

func register(s scraper.Source) {
	s.Langs, s.Sites = []string{"fr"}, SitesJSON
	scraper.Sources = append(scraper.Sources, s)
}

// audioRank: French audio first, then original with French subtitles. An unknown tag counts as French:
// French sites only label the exceptions.
func audioRank(tag string) int {
	switch strings.ToUpper(tag) {
	case "", "VF", "MULTI", "TRUEFRENCH", "FRENCH", "VFF", "VFQ":
		return 2
	case "VOSTFR":
		return 1
	}
	return 0
}
