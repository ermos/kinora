package api

import (
	"testing"

	"github.com/ermos/kinora/internal/tmdb"
)

func TestMarkTop10(t *testing.T) {
	rows := []row{
		{Items: []tmdb.Item{{ID: 1, Type: "tv"}, {ID: 2, Type: "movie", Badge: "new"}, {ID: 3, Type: "movie"}}},
		{Ranked: true, Items: []tmdb.Item{{ID: 1, Type: "tv"}, {ID: 2, Type: "movie"}}},
		{Items: []tmdb.Item{{ID: 1, Type: "movie"}}}, // same ID, other type
	}
	markTop10(rows)
	got := []string{rows[0].Items[0].Badge, rows[0].Items[1].Badge, rows[0].Items[2].Badge, rows[1].Items[0].Badge, rows[2].Items[0].Badge}
	want := []string{"top10", "top10", "", "", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("badges = %q, want %q", got, want)
		}
	}
}
