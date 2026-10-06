package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ermos/kinora/internal/tmdb"
)

func TestNewEpisodes(t *testing.T) {
	conn := testDB(t)
	s, ctx := New(conn), context.Background()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	day := func(d int) string { return now.AddDate(0, 0, d).Format(time.DateOnly) }
	u, _ := s.CreateUser(ctx, "a", "h", false)
	p, _ := s.CreateProfile(ctx, u.ID, "a", "red", true)

	watch := func(id, season, episode int, position float64) {
		t.Helper()
		if err := s.SaveProgress(ctx, p.ID, Progress{Type: "tv", ID: id, Season: season, Episode: episode, Position: position, Duration: 100}); err != nil {
			t.Fatal(err)
		}
	}
	show := func(id int, a tmdb.Airing, checked time.Time) {
		t.Helper()
		if err := s.SaveShow(ctx, id, a, checked); err != nil {
			t.Fatal(err)
		}
	}
	tenEach := []int{0, 10, 10}

	watch(10, 1, 7, 99)
	watch(10, 1, 8, 99) // caught up, episode 9 aired yesterday
	show(10, tmdb.Airing{Status: "Returning Series", LastSeason: 1, LastEpisode: 9, LastAirDate: day(-1), NextAirDate: day(6), Episodes: tenEach}, now)
	watch(11, 1, 10, 99) // finished season 1, season 2 premiered 2 months ago
	show(11, tmdb.Airing{Status: "Ended", LastSeason: 2, LastEpisode: 1, LastAirDate: day(-60), Episodes: tenEach}, now.AddDate(0, 0, -10))
	watch(12, 1, 5, 99) // left in the middle of season 1: not news
	show(12, tmdb.Airing{Status: "Returning Series", LastSeason: 2, LastEpisode: 3, LastAirDate: day(-2), NextAirDate: day(-1), Episodes: tenEach}, now.AddDate(0, 0, -1))
	watch(13, 1, 3, 99) // up to date
	show(13, tmdb.Airing{Status: "Returning Series", LastSeason: 1, LastEpisode: 3, LastAirDate: day(-3), Episodes: tenEach}, now.AddDate(0, 0, -10))
	watch(14, 1, 2, 30) // still watching, never checked

	got, err := s.NewEpisodes(ctx, p.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 ||
		got[0].ID != 10 || got[0].Season != 1 || got[0].Episode != 9 || got[0].Badge != "newEpisode" || !got[0].Recent ||
		got[1].ID != 11 || got[1].Season != 2 || got[1].Episode != 1 || got[1].Badge != "newSeason" || got[1].Recent {
		t.Fatalf("NewEpisodes = %+v", got)
	}

	// 10 checked now, 11 ended and checked 10 days ago: not due. 12's next episode aired, 13 not checked for a week,
	// 14 never checked.
	due, err := s.ShowsToCheck(ctx, now)
	slices.Sort(due)
	if err != nil || !slices.Equal(due, []int{12, 13, 14}) {
		t.Fatalf("ShowsToCheck = %v, %v", due, err)
	}
}
