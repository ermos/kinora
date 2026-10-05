package api

// rowTitles translates the home rows. A language listed in scraper.Languages needs its entry here.
var rowTitles = map[string]map[string]string{
	"fr": {
		"trending":       "Tendances",
		"popularMovies":  "Films populaires",
		"popularShows":   "Séries populaires",
		"topRatedMovies": "Films les mieux notés",
		"action":         "Action",
		"crimeShows":     "Séries policières",
		"comedies":       "Comédies",
		"animation":      "Animation",
		"scifi":          "Science-fiction",
		"horror":         "Horreur",
		"nowPlaying":     "Au cinéma",
		"popular":        "Populaires",
		"topRated":       "Les mieux notés",
		"topRatedShows":  "Les mieux notées",
		"dramas":         "Drames",
		"thrillers":      "Thrillers",
		"crime":          "Policier",
		"scifiFantasy":   "Science-fiction et fantastique",
		"documentaries":  "Documentaires",
	},
}

func rowTitle(lang, key string) string {
	if t, ok := rowTitles[lang][key]; ok {
		return t
	}
	return key
}
