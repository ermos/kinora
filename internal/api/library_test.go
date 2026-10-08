package api

import (
	"testing"

	"github.com/ermos/kinora/internal/store"
)

func TestValidProgress(t *testing.T) {
	for _, tc := range []struct {
		p    store.Progress
		want bool
	}{
		{store.Progress{Type: "movie", ID: 1, Position: 10, Duration: 100}, true},
		{store.Progress{Type: "tv", ID: 1, Season: 1, Episode: 2, Position: 0, Duration: 100}, true},
		{store.Progress{Type: "tv", ID: 1, Season: 0, Episode: 1, Duration: 100}, false},
		{store.Progress{Type: "tv", ID: 1, Season: 1, Episode: -1, Duration: 100}, false},
		{store.Progress{Type: "movie", ID: 1, Season: 1, Episode: 1, Duration: 100}, false},
		{store.Progress{Type: "movie", ID: 1, Duration: 0}, false},
		{store.Progress{Type: "book", ID: 1, Duration: 100}, false},
	} {
		if got := validProgress(tc.p); got != tc.want {
			t.Errorf("validProgress(%+v) = %v, want %v", tc.p, got, tc.want)
		}
	}
}
