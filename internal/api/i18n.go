package api

// rowTitles translates the home rows. A language listed in scraper.Languages needs its entry here.
var rowTitles = map[string]map[string]string{
	"en": {
		"trending":    "Trending",
		"top10Movies": "Top 10 movies today",
		"top10Shows":  "Top 10 shows today",
		// becauseYouWatched takes the title of a watched show or movie.
		"becauseYouWatched": "Because you watched %s",
		"popularMovies":     "Popular movies",
		"popularShows":      "Popular shows",
		"topRatedMovies":    "Top rated movies",
		"action":            "Action",
		"crimeShows":        "Crime shows",
		"comedies":          "Comedies",
		"animation":         "Animation",
		"scifi":             "Science fiction",
		"horror":            "Horror",
		"nowPlaying":        "In theaters",
		"popular":           "Popular",
		"topRated":          "Top rated",
		"topRatedShows":     "Top rated",
		"dramas":            "Dramas",
		"thrillers":         "Thrillers",
		"crime":             "Crime",
		"scifiFantasy":      "Sci-fi and fantasy",
		"documentaries":     "Documentaries",
	},
	"fr": {
		"trending":          "Tendances",
		"top10Movies":       "Top 10 des films aujourd'hui",
		"top10Shows":        "Top 10 des séries aujourd'hui",
		"becauseYouWatched": "Parce que vous avez regardé %s",
		"popularMovies":     "Films populaires",
		"popularShows":      "Séries populaires",
		"topRatedMovies":    "Films les mieux notés",
		"action":            "Action",
		"crimeShows":        "Séries policières",
		"comedies":          "Comédies",
		"animation":         "Animation",
		"scifi":             "Science-fiction",
		"horror":            "Horreur",
		"nowPlaying":        "Au cinéma",
		"popular":           "Populaires",
		"topRated":          "Les mieux notés",
		"topRatedShows":     "Les mieux notées",
		"dramas":            "Drames",
		"thrillers":         "Thrillers",
		"crime":             "Policier",
		"scifiFantasy":      "Science-fiction et fantastique",
		"documentaries":     "Documentaires",
	},
}

func rowTitle(lang, key string) string {
	if t, ok := rowTitles[lang][key]; ok {
		return t
	}
	return key
}
