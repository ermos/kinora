package aniskip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolve(t *testing.T) {
	index := map[key][]entry{
		// Frieren: TMDB keeps the second cour in season 1, after episode 28.
		{"tv", 209867}: {{mal: 52991, season: 1}, {mal: 59978, season: 1, offset: 28}},
		{"tv", 1}:      {{mal: 100}}, // season unknown
		{"movie", 2}:   {{mal: 200}},
	}
	cases := []struct {
		kind            string
		id, season, ep  int
		wantMAL, wantEp int
		wantOK          bool
	}{
		{"tv", 209867, 1, 3, 52991, 3, true},
		{"tv", 209867, 1, 29, 59978, 1, true},
		{"tv", 209867, 2, 1, 0, 0, false},
		{"tv", 1, 1, 5, 100, 5, true},
		{"tv", 1, 2, 5, 0, 0, false},
		{"movie", 2, 0, 0, 200, 1, true},
		{"tv", 404, 1, 1, 0, 0, false},
	}
	for _, c := range cases {
		mal, ep, ok := resolve(index, c.kind, c.id, c.season, c.ep)
		if mal != c.wantMAL || ep != c.wantEp || ok != c.wantOK {
			t.Errorf("resolve(%s %d S%dE%d) = %d, %d, %v", c.kind, c.id, c.season, c.ep, mal, ep, ok)
		}
	}
}

func TestSegments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/list":
			w.Write([]byte(`[{"mal_id":52991,"themoviedb_id":{"tv":209867},"season":{"tvdb":1,"tmdb":1}},{"mal_id":5,"anidb_id":9},{"mal_id":6,"themoviedb_id":{"movie":[7,8]}}]`))
		case "/skip/52991/3":
			w.Write([]byte(`{"found":true,"results":[{"interval":{"startTime":1.5,"endTime":91.5},"skipType":"op"},{"interval":{"startTime":1367,"endTime":1459},"skipType":"ed"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New()
	c.listURL, c.apiURL = srv.URL+"/list", srv.URL+"/skip/%d/%d?len=%d"

	got, err := c.Segments(context.Background(), "tv", 209867, 1, 3, 1470)
	if err != nil || len(got) != 2 || got[0] != (Segment{"intro", 1.5, 91.5}) || got[1] != (Segment{"credits", 1367, 1459}) {
		t.Fatalf("Segments = %v, %v", got, err)
	}
	if _, _, ok := resolve(c.index, "movie", 8, 0, 0); !ok {
		t.Fatal("movie listed in an ID array not indexed")
	}
	if got, err := c.Segments(context.Background(), "tv", 209867, 1, 4, 1470); err != nil || len(got) != 0 {
		t.Fatalf("untimed episode = %v, %v", got, err)
	}
}
