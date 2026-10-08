package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ermos/kinora/internal/aniskip"
	"github.com/ermos/kinora/internal/scraper"
	"github.com/ermos/kinora/internal/stream"
	"github.com/ermos/kinora/internal/tmdb"
)

type row struct {
	Title string      `json:"title"`
	Items []tmdb.Item `json:"items"`
	// Ranked rows are Top 10s: the UI draws the rank next to each poster.
	Ranked bool `json:"ranked,omitempty"`
}

// rowSpec.key is translated by rowTitles (i18n.go) in the instance language.
type rowSpec struct {
	key, kind, path string
	params          url.Values
}

// top10Row is today's most watched titles of a kind, like Netflix's Top 10 rows.
func top10Row(kind string) rowSpec {
	key := map[string]string{"movie": "top10Movies", "tv": "top10Shows"}[kind]
	return rowSpec{key, kind, "trending/" + kind + "/day", nil}
}

func (s rowSpec) ranked() bool { return strings.HasPrefix(s.key, "top10") }

func genreRow(key, kind string, genre int) rowSpec {
	return rowSpec{key, kind, "discover/" + kind, url.Values{"with_genres": {strconv.Itoa(genre)}, "sort_by": {"popularity.desc"}}}
}

var homeRows = map[string][]rowSpec{
	"": {
		{"trending", "", "trending/all/week", nil},
		top10Row("tv"),
		{"popularMovies", "movie", "movie/popular", nil},
		top10Row("movie"),
		{"popularShows", "tv", "tv/popular", nil},
		{"topRatedMovies", "movie", "movie/top_rated", nil},
		genreRow("action", "movie", 28),
		genreRow("crimeShows", "tv", 80),
		genreRow("comedies", "movie", 35),
		genreRow("animation", "tv", 16),
		genreRow("scifi", "movie", 878),
		genreRow("horror", "movie", 27),
	},
	"movie": {
		{"trending", "movie", "trending/movie/week", nil},
		top10Row("movie"),
		{"nowPlaying", "movie", "movie/now_playing", nil},
		{"popular", "movie", "movie/popular", nil},
		{"topRated", "movie", "movie/top_rated", nil},
		genreRow("action", "movie", 28),
		genreRow("comedies", "movie", 35),
		genreRow("dramas", "movie", 18),
		genreRow("animation", "movie", 16),
		genreRow("scifi", "movie", 878),
		genreRow("thrillers", "movie", 53),
		genreRow("horror", "movie", 27),
	},
	"tv": {
		{"trending", "tv", "trending/tv/week", nil},
		top10Row("tv"),
		{"popular", "tv", "tv/popular", nil},
		{"topRatedShows", "tv", "tv/top_rated", nil},
		genreRow("crime", "tv", 80),
		genreRow("dramas", "tv", 18),
		genreRow("comedies", "tv", 35),
		genreRow("animation", "tv", 16),
		genreRow("scifiFantasy", "tv", 10765),
		genreRow("documentaries", "tv", 99),
	},
}

// @Summary  Home rows (all, movies or shows)
// @Tags     catalog
// @Param    type  query  string  false  "movie or tv, empty for both"
// @Success  200   {array}  row
// @Router   /catalog/home [get]
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	specs, ok := homeRows[r.URL.Query().Get("type")]
	if !ok {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	lang := h.language().Code
	rows := make([]row, len(specs))
	var wg sync.WaitGroup
	for i, s := range specs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := h.tmdb.List(r.Context(), s.kind, s.path, s.params)
			if err != nil {
				slog.Warn("tmdb row failed", "row", s.key, "err", err)
			}
			if s.ranked() && len(items) > 10 {
				items = items[:10]
			}
			rows[i] = row{Title: rowTitle(lang, s.key), Items: items, Ranked: s.ranked()}
		}()
	}
	wg.Wait()
	out := rows[:0]
	for _, rw := range rows {
		if len(rw.Items) > 0 {
			out = append(out, rw)
		}
	}
	if len(out) == 0 {
		writeError(w, http.StatusBadGateway, errTMDBUnreachable)
		return
	}
	markTop10(out)
	h.addHeroLogos(r.Context(), out[0].Items)
	writeJSON(w, http.StatusOK, out)
}

// markTop10 badges the titles of the other rows that are in one of the page's Top 10 rows.
func markTop10(rows []row) {
	top := map[string]bool{}
	for _, rw := range rows {
		for _, it := range rw.Items {
			if rw.Ranked {
				top[it.Type+strconv.Itoa(it.ID)] = true
			}
		}
	}
	for _, rw := range rows {
		for i, it := range rw.Items {
			if !rw.Ranked && top[it.Type+strconv.Itoa(it.ID)] {
				rw.Items[i].Badge = "top10"
			}
		}
	}
}

// personalSeeds is how many recently watched titles get their "Because you watched" row.
const personalSeeds = 3

// @Summary  Rows personalized for the profile: "Because you watched X", from TMDB recommendations of recent titles
// @Tags     catalog
// @Security ProfileHeader
// @Param    type  query  string  false  "movie or tv, empty for both"
// @Success  200   {array}  row
// @Router   /catalog/foryou [get]
func (h *Handler) forYou(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("type")
	if kind != "" && kind != "movie" && kind != "tv" {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	watched, err := h.store.RecentlyWatched(r.Context(), currentProfile(r), kind, 100)
	if err != nil {
		internalError(w, err)
		return
	}
	seeds := watched[:min(personalSeeds, len(watched))]
	recs := make([][]tmdb.Item, len(seeds))
	var wg sync.WaitGroup
	for i, s := range seeds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := h.tmdb.Recommendations(r.Context(), s.Type, s.ID)
			if err != nil {
				slog.Warn("tmdb recommendations failed", "type", s.Type, "id", s.ID, "err", err)
			}
			recs[i] = items
		}()
	}
	wg.Wait()

	seen := map[string]bool{}
	for _, wt := range watched {
		seen[wt.Type+strconv.Itoa(wt.ID)] = true
	}
	lang := h.language().Code
	rows := []row{}
	for i, s := range seeds {
		if items := freshPicks(recs[i], seen); len(items) > 0 {
			rows = append(rows, row{Title: fmt.Sprintf(rowTitle(lang, "becauseYouWatched"), s.Title), Items: items})
		}
	}
	writeJSON(w, http.StatusOK, rows)
}

// freshPicks keeps the titles not yet played nor shown in an earlier row, and marks them as shown.
func freshPicks(items []tmdb.Item, seen map[string]bool) []tmdb.Item {
	var out []tmdb.Item
	for _, it := range items {
		k := it.Type + strconv.Itoa(it.ID)
		if !seen[k] {
			seen[k] = true
			out = append(out, it)
		}
	}
	return out
}

// heroCount is how many titles of the first row the UI banner cycles through (HERO_COUNT in Browse.tsx).
const heroCount = 8

// addHeroLogos sets the title artwork of the banner candidates: first-row titles with a backdrop and an overview.
// A missing logo leaves the title as text.
func (h *Handler) addHeroLogos(ctx context.Context, items []tmdb.Item) {
	var wg sync.WaitGroup
	n := 0
	for i := range items {
		if n == heroCount {
			break
		}
		if items[i].Backdrop == "" || items[i].Overview == "" {
			continue
		}
		n++
		wg.Add(1)
		go func() {
			defer wg.Done()
			if logo, err := h.tmdb.Logo(ctx, items[i].Type, items[i].ID); err == nil {
				items[i].Logo = logo
			}
		}()
	}
	wg.Wait()
}

// @Summary  Genres of movies or shows
// @Tags     catalog
// @Param    type  query  string  true  "movie or tv"
// @Success  200   {array}  tmdb.Genre
// @Router   /catalog/genres [get]
func (h *Handler) genres(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	if !ok {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	gs, err := h.tmdb.Genres(r.Context(), kind)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, gs)
}

// @Summary  Titles of a genre, most popular first
// @Tags     catalog
// @Param    type   query  string  true   "movie or tv"
// @Param    genre  query  int     true   "TMDB genre ID"
// @Param    page   query  int     false  "Page, from 1"
// @Success  200    {array}  tmdb.Item
// @Router   /catalog/discover [get]
func (h *Handler) discover(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	genre, err := strconv.Atoi(r.URL.Query().Get("genre"))
	if !ok || err != nil {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	items, err := h.tmdb.Discover(r.Context(), kind, genre, max(page, 1))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// @Summary  Search movies and shows
// @Tags     catalog
// @Param    q  query  string  true  "Query"
// @Success  200  {array}  tmdb.Item
// @Router   /catalog/search [get]
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []tmdb.Item{})
		return
	}
	items, err := h.tmdb.Search(r.Context(), q)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// @Summary  Details of a movie or show
// @Tags     catalog
// @Param    type  path  string  true  "movie or tv"
// @Param    id    path  int     true  "TMDB ID"
// @Success  200   {object}  tmdb.Details
// @Router   /titles/{type}/{id} [get]
func (h *Handler) title(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	id, ok2 := pathInt(r, "id")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	d, err := h.tmdb.Details(r.Context(), kind, id)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// @Summary  Opening and ending of a movie or episode, for the skip buttons (empty when unknown or turned off for the profile)
// @Tags     catalog
// @Security ProfileHeader
// @Param    type      path   string  true   "movie or tv"
// @Param    id        path   int     true   "TMDB ID"
// @Param    season    query  int     false  "Season (shows)"
// @Param    episode   query  int     false  "Episode (shows)"
// @Param    duration  query  number  true   "Length of the file being played, in seconds"
// @Success  200  {array}  aniskip.Segment
// @Router   /titles/{type}/{id}/segments [get]
func (h *Handler) segments(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	id, ok2 := pathInt(r, "id")
	q := r.URL.Query()
	season, _ := strconv.Atoi(q.Get("season"))
	episode, _ := strconv.Atoi(q.Get("episode"))
	duration, err := strconv.ParseFloat(q.Get("duration"), 64)
	if !ok || !ok2 || err != nil || duration <= 0 || (kind == "tv" && (season < 1 || episode < 1)) {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	segments := []aniskip.Segment{}
	if on, err := h.store.ProfileSkipSegments(r.Context(), currentProfile(r)); err != nil {
		internalError(w, err)
		return
	} else if on {
		found, err := h.aniskip.Segments(r.Context(), kind, id, season, episode, duration)
		if err != nil {
			slog.Warn("aniskip failed", "err", err) // skip buttons are a bonus: play on without them
		}
		segments = append(segments, found...)
	}
	writeJSON(w, http.StatusOK, segments)
}

// @Summary  Episodes of a season
// @Tags     catalog
// @Param    id      path  int  true  "TMDB ID"
// @Param    season  path  int  true  "Season number"
// @Success  200     {array}  tmdb.Episode
// @Router   /titles/tv/{id}/seasons/{season} [get]
func (h *Handler) season(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	n, ok2 := pathInt(r, "season")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	eps, err := h.tmdb.Season(r.Context(), id, n)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eps)
}

// link is a playable link; token is opaque and signed, pass it to /play. Links come sorted best first
// (French audio, then measured quality); dead ones failed to resolve while probing and come last.
type link struct {
	Token   string `json:"token"`
	Source  string `json:"source"`
	Hoster  string `json:"hoster"`
	Lang    string `json:"lang"`
	Quality string `json:"quality"`
	Dead    bool   `json:"dead"`
}

// @Summary  Find links for a movie or an episode across the sources of the instance language
// @Tags     playback
// @Param    type     path   string  true   "movie or tv"
// @Param    id       path   int     true   "TMDB ID"
// @Param    season   query  int     false  "Season (shows)"
// @Param    episode  query  int     false  "Episode (shows)"
// @Success  200      {array}  link
// @Router   /titles/{type}/{id}/links [get]
func (h *Handler) links(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	id, ok2 := pathInt(r, "id")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	d, err := h.tmdb.Details(r.Context(), kind, id)
	if err != nil {
		internalError(w, err)
		return
	}
	q := scraper.Query{Lang: h.language().Code, Type: kind, TMDBID: id, Title: d.Title, OriginalTitle: d.OriginalTitle, AltTitles: d.AltTitles, Year: d.Year}
	if kind == "tv" {
		q.Season, _ = strconv.Atoi(r.URL.Query().Get("season"))
		q.Episode, _ = strconv.Atoi(r.URL.Query().Get("episode"))
		if q.Season < 1 || q.Episode < 1 {
			writeError(w, http.StatusBadRequest, errEpisodeRequired)
			return
		}
	}
	found, ok := h.linkCache.get(q)
	if !ok {
		found = scraper.FindLinks(r.Context(), func(id string) string {
			return h.store.SourceURL(r.Context(), id)
		}, q, h.language())
		if r.Context().Err() == nil {
			h.linkCache.put(q, found)
		}
	}
	out := make([]link, 0, len(found))
	exp := time.Now().Add(linkTokenTTL).Unix()
	for _, l := range found {
		tok, err := h.signer.Sign(linkTokenKind, linkToken{Link: l, Exp: exp})
		if err != nil {
			internalError(w, err)
			return
		}
		out = append(out, link{Token: tok, Source: l.Source, Hoster: l.Hoster, Lang: l.Lang, Quality: l.Quality, Dead: l.Dead})
	}
	writeJSON(w, http.StatusOK, out)
}

const (
	linkTokenKind = "link"
	// linkTokenTTL bounds how long a link from /links can be played: long enough for a page left open.
	linkTokenTTL = 6 * time.Hour
)

// linkToken is what a link token carries: the link and when it stops being playable.
type linkToken struct {
	scraper.Link
	Exp int64 `json:"exp"`
}

type playRequest struct {
	Token string `json:"token"`
}

type playResponse struct {
	URL  string `json:"url"`
	Kind string `json:"kind" enums:"hls,file"`
}

// @Summary  Resolve a link into a proxied stream URL
// @Tags     playback
// @Param    body  body      playRequest  true  "Link token"
// @Success  200   {object}  playResponse
// @Failure  502   {object}  apiError
// @Router   /play [post]
func (h *Handler) play(w http.ResponseWriter, r *http.Request) {
	var req playRequest
	if !readJSON(w, r, &req) {
		return
	}
	var lt linkToken
	if err := h.signer.Verify(linkTokenKind, req.Token, &lt); err != nil || time.Now().Unix() > lt.Exp {
		writeError(w, http.StatusBadRequest, errInvalidLink)
		return
	}
	l := lt.Link
	st, err := scraper.Resolve(r.Context(), l)
	if err != nil {
		slog.Warn("resolve failed", "hoster", l.Hoster, "url", l.URL, "err", err)
		writeError(w, http.StatusBadGateway, errLinkDead)
		return
	}
	u, err := h.proxy.URL(st, currentUser(r).MaxStreamMbps)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, playResponse{URL: u, Kind: stream.Kind(st.URL)})
}

// linkCache keeps the probed links of a title for a few minutes: reopening the player, going back to an
// episode or switching profile does not search and probe every source again.
// ponytail: in-memory, unbounded between sweeps; entries are small and expire quickly.
type linkCache struct {
	mu      sync.Mutex
	entries map[string]cachedLinks
}

type cachedLinks struct {
	links   []scraper.Link
	expires time.Time
}

const linkCacheTTL = 10 * time.Minute

func (c *linkCache) key(q scraper.Query) string {
	return fmt.Sprintf("%s/%s/%d/%d/%d", q.Lang, q.Type, q.TMDBID, q.Season, q.Episode)
}

func (c *linkCache) get(q scraper.Query) ([]scraper.Link, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[c.key(q)]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.links, true
}

func (c *linkCache) put(q scraper.Query, links []scraper.Link) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cachedLinks{}
	}
	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expires) {
			delete(c.entries, k)
		}
	}
	c.entries[c.key(q)] = cachedLinks{links: links, expires: now.Add(linkCacheTTL)}
}
