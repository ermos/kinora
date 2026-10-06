// Package tmdb is a small TMDB v3 client: the catalog (rows, details, seasons) comes from TMDB,
// sources are only asked for links.
package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	key  string
	lang atomic.Value // string, TMDB locale ("fr-FR")
	http *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	body    []byte
	expires time.Time
}

func New(key, lang string) *Client {
	c := &Client{key: key, http: &http.Client{Timeout: 15 * time.Second}, cache: map[string]cached{}}
	c.SetLanguage(lang)
	return c
}

// SetLanguage switches the catalog language; cached answers are keyed by URL, so per language.
func (c *Client) SetLanguage(lang string) { c.lang.Store(lang) }

type Item struct {
	ID       int     `json:"id"`
	Type     string  `json:"type"`
	Title    string  `json:"title"`
	Overview string  `json:"overview"`
	Poster   string  `json:"poster"`
	Backdrop string  `json:"backdrop"`
	Year     int     `json:"year"`
	Rating   float64 `json:"rating"`
	// Logo is the title artwork (transparent PNG), set on the home banner titles and on details.
	Logo string `json:"logo,omitempty"`
	// Badge is the Netflix-style tag shown on cards: in today's Top 10, newly released, or a show with a new
	// episode or season (details only).
	Badge string `json:"badge,omitempty" enums:"top10,new,newEpisode,newSeason"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Season struct {
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Episodes int    `json:"episodes"`
	Poster   string `json:"poster"`
}

type Details struct {
	Item
	OriginalTitle string   `json:"originalTitle"`
	Runtime       int      `json:"runtime"`
	Genres        []Genre  `json:"genres"`
	Seasons       []Season `json:"seasons"`
	Cast          []string `json:"cast"`
	Similar       []Item   `json:"similar"`
	// AltTitles feed the source search (romaji and English names of animes...), not the UI.
	AltTitles []string `json:"-"`
}

type Episode struct {
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Overview string `json:"overview"`
	Still    string `json:"still"`
	Runtime  int    `json:"runtime"`
}

// raw is the TMDB shape shared by movies and shows (movies use title/release_date, shows name/first_air_date).
type raw struct {
	ID            int     `json:"id"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	VoteAverage   float64 `json:"vote_average"`
}

func (r raw) item(kind string) Item {
	if r.MediaType != "" {
		kind = r.MediaType
	}
	it := Item{ID: r.ID, Type: kind, Title: r.Title, Overview: r.Overview, Poster: r.PosterPath, Backdrop: r.BackdropPath, Rating: r.VoteAverage}
	date := r.ReleaseDate
	if kind == "tv" {
		it.Title, date = r.Name, r.FirstAirDate
	}
	if len(date) >= 4 {
		it.Year, _ = strconv.Atoi(date[:4])
	}
	if recent(date, newDays, time.Now()) {
		it.Badge = "new"
	}
	return it
}

const (
	newDays        = 30 // a title counts as new for a month after its release
	newEpisodeDays = 14
)

// recent reports whether a TMDB date ("2006-01-02") falls in the last days, today included; future dates don't.
func recent(date string, days int, now time.Time) bool {
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return false
	}
	age := now.Sub(d)
	return age >= 0 && age < time.Duration(days+1)*24*time.Hour
}

type page struct {
	Results []raw `json:"results"`
}

func items(p page, kind string) []Item {
	out := make([]Item, 0, len(p.Results))
	for _, r := range p.Results {
		it := r.item(kind)
		if (it.Type == "movie" || it.Type == "tv") && it.Poster != "" {
			out = append(out, it)
		}
	}
	return out
}

// List fetches a paged list endpoint such as "trending/movie/week" or "movie/top_rated".
func (c *Client) List(ctx context.Context, kind, path string, params url.Values) ([]Item, error) {
	var p page
	if err := c.get(ctx, path, params, &p); err != nil {
		return nil, err
	}
	return items(p, kind), nil
}

func (c *Client) Discover(ctx context.Context, kind string, genre, pageNum int) ([]Item, error) {
	return c.List(ctx, kind, "discover/"+kind, url.Values{
		"with_genres": {strconv.Itoa(genre)}, "sort_by": {"popularity.desc"}, "page": {strconv.Itoa(pageNum)},
	})
}

// Recommendations are the titles TMDB suggests to people who liked this one.
func (c *Client) Recommendations(ctx context.Context, kind string, id int) ([]Item, error) {
	return c.List(ctx, kind, fmt.Sprintf("%s/%d/recommendations", kind, id), nil)
}

func (c *Client) Search(ctx context.Context, query string) ([]Item, error) {
	return c.List(ctx, "", "search/multi", url.Values{"query": {query}})
}

func (c *Client) Genres(ctx context.Context, kind string) ([]Genre, error) {
	var r struct {
		Genres []Genre `json:"genres"`
	}
	err := c.get(ctx, "genre/"+kind+"/list", nil, &r)
	return r.Genres, err
}

// Airing is where a show stands: its latest aired episode, the date of the next one if announced.
type Airing struct {
	// Status is TMDB's: "Returning Series", "In Production", "Ended", "Canceled"...
	Status      string
	LastSeason  int
	LastEpisode int
	LastAirDate string // "2006-01-02", "" before the first episode
	NextAirDate string
	// Episodes[n] is the episode count of season n (0 holds specials).
	Episodes []int
}

// Airing fetches a show without the extras Details appends, for the background new episodes check.
func (c *Client) Airing(ctx context.Context, id int) (Airing, error) {
	var r struct {
		Status  string `json:"status"`
		Seasons []struct {
			SeasonNumber int `json:"season_number"`
			EpisodeCount int `json:"episode_count"`
		} `json:"seasons"`
		LastEp *struct {
			AirDate string `json:"air_date"`
			Episode int    `json:"episode_number"`
			Season  int    `json:"season_number"`
		} `json:"last_episode_to_air"`
		NextEp *struct {
			AirDate string `json:"air_date"`
		} `json:"next_episode_to_air"`
	}
	if err := c.get(ctx, fmt.Sprintf("tv/%d", id), nil, &r); err != nil {
		return Airing{}, err
	}
	a := Airing{Status: r.Status}
	if r.LastEp != nil {
		a.LastSeason, a.LastEpisode, a.LastAirDate = r.LastEp.Season, r.LastEp.Episode, r.LastEp.AirDate
	}
	if r.NextEp != nil {
		a.NextAirDate = r.NextEp.AirDate
	}
	for _, s := range r.Seasons {
		if s.SeasonNumber < 0 {
			continue
		}
		for len(a.Episodes) <= s.SeasonNumber {
			a.Episodes = append(a.Episodes, 0)
		}
		a.Episodes[s.SeasonNumber] = s.EpisodeCount
	}
	return a, nil
}

func (c *Client) Details(ctx context.Context, kind string, id int) (Details, error) {
	var r struct {
		raw
		Runtime        int     `json:"runtime"`
		EpisodeRunTime []int   `json:"episode_run_time"`
		Genres         []Genre `json:"genres"`
		Seasons        []struct {
			SeasonNumber int    `json:"season_number"`
			Name         string `json:"name"`
			EpisodeCount int    `json:"episode_count"`
			PosterPath   string `json:"poster_path"`
		} `json:"seasons"`
		Credits struct {
			Cast []struct {
				Name string `json:"name"`
			} `json:"cast"`
		} `json:"credits"`
		Similar page   `json:"similar"`
		Images  images `json:"images"`
		LastEp  struct {
			AirDate string `json:"air_date"`
			Episode int    `json:"episode_number"`
			Season  int    `json:"season_number"`
		} `json:"last_episode_to_air"`
		Alt struct {
			Titles  []altTitle `json:"titles"`  // movies
			Results []altTitle `json:"results"` // shows
		} `json:"alternative_titles"`
	}
	params := url.Values{"append_to_response": {"credits,similar,alternative_titles,images"}, "include_image_language": {c.imageLanguages()}}
	if err := c.get(ctx, fmt.Sprintf("%s/%d", kind, id), params, &r); err != nil {
		return Details{}, err
	}
	d := Details{Item: r.item(kind), OriginalTitle: r.OriginalTitle, Runtime: r.Runtime, Genres: r.Genres, Similar: items(r.Similar, kind)}
	d.Logo = r.Images.logo(c.lang2())
	// A show still airing: its latest episode makes the badge, a premiere opens a new season. Brand-new shows stay "new".
	if kind == "tv" && d.Badge == "" && recent(r.LastEp.AirDate, newEpisodeDays, time.Now()) {
		d.Badge = "newEpisode"
		if r.LastEp.Episode == 1 && r.LastEp.Season > 1 {
			d.Badge = "newSeason"
		}
	}
	for _, s := range r.Seasons {
		if s.SeasonNumber > 0 && s.EpisodeCount > 0 { // season 0 holds specials
			d.Seasons = append(d.Seasons, Season{Number: s.SeasonNumber, Name: s.Name, Episodes: s.EpisodeCount, Poster: s.PosterPath})
		}
	}
	d.AltTitles = pickAltTitles(append(r.Alt.Titles, r.Alt.Results...))
	for i, a := range r.Credits.Cast {
		if i == 6 {
			break
		}
		d.Cast = append(d.Cast, a.Name)
	}
	return d, nil
}

type images struct {
	Logos []struct {
		Lang     string `json:"iso_639_1"`
		FilePath string `json:"file_path"`
	} `json:"logos"`
}

// logo picks the title artwork in the catalog language, else in English, else one without text.
// SVG logos are skipped: not every player of the UI renders them.
func (im images) logo(lang string) string {
	for _, want := range []string{lang, "en", ""} {
		for _, l := range im.Logos {
			if l.Lang == want && !strings.HasSuffix(l.FilePath, ".svg") {
				return l.FilePath
			}
		}
	}
	return ""
}

// Logo returns the title artwork of a movie or show ("" when TMDB has none).
func (c *Client) Logo(ctx context.Context, kind string, id int) (string, error) {
	var im images
	if err := c.get(ctx, fmt.Sprintf("%s/%d/images", kind, id), url.Values{"include_image_language": {c.imageLanguages()}}, &im); err != nil {
		return "", err
	}
	return im.logo(c.lang2()), nil
}

// lang2 is the ISO 639-1 part of the catalog locale ("fr-FR" -> "fr"), the language TMDB tags images with.
func (c *Client) lang2() string {
	l, _, _ := strings.Cut(c.lang.Load().(string), "-")
	return l
}

func (c *Client) imageLanguages() string { return c.lang2() + ",en,null" }

func (c *Client) Season(ctx context.Context, id, number int) ([]Episode, error) {
	var r struct {
		Episodes []struct {
			EpisodeNumber int    `json:"episode_number"`
			Name          string `json:"name"`
			Overview      string `json:"overview"`
			StillPath     string `json:"still_path"`
			Runtime       int    `json:"runtime"`
		} `json:"episodes"`
	}
	if err := c.get(ctx, fmt.Sprintf("tv/%d/season/%d", id, number), nil, &r); err != nil {
		return nil, err
	}
	out := make([]Episode, len(r.Episodes))
	for i, e := range r.Episodes {
		out[i] = Episode{Number: e.EpisodeNumber, Name: e.Name, Overview: e.Overview, Still: e.StillPath, Runtime: e.Runtime}
	}
	return out, nil
}

// ponytail: in-memory cache with a fixed 6h TTL and no eviction, entries are small and keys bounded by what users browse.
func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("language", c.lang.Load().(string))
	u := "https://api.themoviedb.org/3/" + path + "?" + params.Encode()

	c.mu.Lock()
	hit, ok := c.cache[u]
	c.mu.Unlock()
	if ok && time.Now().Before(hit.expires) {
		return json.Unmarshal(hit.body, out)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	// v4 read tokens are JWTs, v3 keys are 32 hex chars.
	if len(c.key) > 40 {
		req.Header.Set("Authorization", "Bearer "+c.key)
	} else {
		q := req.URL.Query()
		q.Set("api_key", c.key)
		req.URL.RawQuery = q.Encode()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb %s: status %d", path, resp.StatusCode)
	}
	var body json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	c.mu.Lock()
	c.cache[u] = cached{body: body, expires: time.Now().Add(6 * time.Hour)}
	c.mu.Unlock()
	return json.Unmarshal(body, out)
}

type altTitle struct {
	Title   string `json:"title"`
	Country string `json:"iso_3166_1"`
}

// pickAltTitles keeps a few Latin-script alternative titles, the ones French sites use: romaji (JP),
// English (US/GB), then French variants.
func pickAltTitles(alts []altTitle) []string {
	rank := map[string]int{"JP": 0, "US": 1, "GB": 2, "FR": 3}
	sort.SliceStable(alts, func(i, j int) bool {
		ri, ok := rank[alts[i].Country]
		if !ok {
			ri = 9
		}
		rj, ok := rank[alts[j].Country]
		if !ok {
			rj = 9
		}
		return ri < rj
	})
	var out []string
	for _, a := range alts {
		if len(out) == 3 {
			break
		}
		if a.Title != "" && latin(a.Title) {
			out = append(out, a.Title)
		}
	}
	return out
}

func latin(s string) bool {
	for _, r := range s {
		if r > 0x24F {
			return false
		}
	}
	return true
}
