// Package aniskip finds the opening and ending of anime episodes, for the "skip intro" buttons. AniSkip
// (api.aniskip.com) has community timestamps by MyAnimeList ID; Fribb's anime-lists maps TMDB IDs to MyAnimeList.
package aniskip

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/ermos/kinora/internal/ttlcache"
)

const (
	listURL = "https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-full.json"
	apiURL  = "https://api.aniskip.com/v2/skip-times/%d/%d?types[]=op&types[]=ed&episodeLength=%d"
	// The list grows with each anime season; a day old is fresh enough.
	listTTL  = 24 * time.Hour
	retryTTL = 10 * time.Minute
	skipTTL  = 6 * time.Hour
)

type Segment struct {
	Kind  string  `json:"kind" enums:"intro,credits"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type Client struct {
	http    *http.Client
	listURL string
	apiURL  string

	mu      sync.Mutex
	index   map[key][]entry
	expires time.Time
	// skips is keyed by API URL, which holds the duration the client sends: the size is capped.
	skips *ttlcache.Cache[string, []Segment]
}

type key struct {
	kind string // "tv" or "movie"
	id   int
}

// entry is one MyAnimeList title of a TMDB show: a TMDB season, possibly split in parts (offset).
type entry struct {
	mal    int
	season int // TMDB season, 0 when unknown
	offset int // episodes of the TMDB season before this part
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}, listURL: listURL, apiURL: apiURL, skips: ttlcache.New[string, []Segment](skipTTL, 5000)}
}

// Segments returns the opening and ending of an episode (season and episode are 0 for movies). duration, the
// length of the file being played, lets AniSkip discard timestamps sent for other cuts. Titles that aren't
// anime, or that nobody timed, have none.
func (c *Client) Segments(ctx context.Context, kind string, id, season, episode int, duration float64) ([]Segment, error) {
	index, err := c.list(ctx)
	if err != nil {
		return nil, err
	}
	mal, ep, ok := resolve(index, kind, id, season, episode)
	if !ok {
		return nil, nil
	}
	return c.skipTimes(ctx, fmt.Sprintf(c.apiURL, mal, ep, int(math.Round(duration))))
}

// resolve maps a TMDB title (and episode) to a MyAnimeList title and its episode number.
func resolve(index map[key][]entry, kind string, id, season, episode int) (mal, ep int, ok bool) {
	entries := index[key{kind, id}]
	if kind == "movie" {
		if len(entries) == 0 {
			return 0, 0, false
		}
		return entries[0].mal, 1, true
	}
	var best *entry
	for i, e := range entries {
		// An unknown season only counts for the first one: later seasons would get season 1's timestamps.
		sameSeason := e.season == season || (e.season == 0 && season == 1)
		if sameSeason && e.offset < episode && (best == nil || e.offset > best.offset) {
			best = &entries[i]
		}
	}
	if best == nil {
		return 0, 0, false
	}
	return best.mal, episode - best.offset, true
}

func (c *Client) list(ctx context.Context) (map[key][]entry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expires) {
		return c.index, nil
	}
	index, err := c.fetchList(context.WithoutCancel(ctx)) // shared by the callers waiting on mu, bounded by the client timeout
	if err != nil {
		c.expires = time.Now().Add(retryTTL) // keep the old index, if any, and retry later
		return c.index, err
	}
	c.index, c.expires = index, time.Now().Add(listTTL)
	return index, nil
}

func (c *Client) fetchList(ctx context.Context) (map[key][]entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.listURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anime list: status %d", resp.StatusCode)
	}
	var rows []struct {
		MAL  int `json:"mal_id"`
		TMDB struct {
			TV    ids `json:"tv"`
			Movie ids `json:"movie"`
		} `json:"themoviedb_id"`
		Season struct {
			TMDB int `json:"tmdb"`
		} `json:"season"`
		Offset struct {
			TMDB int `json:"tmdb"`
		} `json:"episode_offset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	index := map[key][]entry{}
	for _, r := range rows {
		if r.MAL == 0 {
			continue
		}
		for _, id := range r.TMDB.TV {
			k := key{"tv", id}
			index[k] = append(index[k], entry{mal: r.MAL, season: r.Season.TMDB, offset: r.Offset.TMDB})
		}
		for _, id := range r.TMDB.Movie {
			k := key{"movie", id}
			index[k] = append(index[k], entry{mal: r.MAL})
		}
	}
	return index, nil
}

// ids reads a TMDB ID that the list gives as a number, or as an array when one MyAnimeList entry spans several.
type ids []int

func (v *ids) UnmarshalJSON(b []byte) error {
	var one int
	if json.Unmarshal(b, &one) == nil {
		*v = ids{one}
		return nil
	}
	return json.Unmarshal(b, (*[]int)(v))
}

func (c *Client) skipTimes(ctx context.Context, u string) ([]Segment, error) {
	if segments, ok := c.skips.Get(u); ok {
		return segments, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Results []struct {
			Interval struct {
				Start float64 `json:"startTime"`
				End   float64 `json:"endTime"`
			} `json:"interval"`
			SkipType string `json:"skipType"`
		} `json:"results"`
	}
	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, err
		}
	case http.StatusNotFound: // nobody timed this episode
	default:
		return nil, fmt.Errorf("aniskip: status %d", resp.StatusCode)
	}
	segments := []Segment{}
	for _, r := range out.Results {
		kind := map[string]string{"op": "intro", "ed": "credits"}[r.SkipType]
		if kind != "" && r.Interval.End > r.Interval.Start {
			segments = append(segments, Segment{Kind: kind, Start: r.Interval.Start, End: r.Interval.End})
		}
	}
	c.skips.Set(u, segments)
	return segments, nil
}
