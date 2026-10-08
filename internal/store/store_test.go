package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ermos/kinora/internal/dbtest"
	"github.com/ermos/kinora/internal/tmdb"
)

func testDB(t *testing.T) *sql.DB { return dbtest.Open(t) }

func TestContinueWatching(t *testing.T) {
	conn := testDB(t)
	s, ctx := New(conn), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	p, _ := s.CreateProfile(ctx, u.ID, "a", "red", true)
	other, _ := s.CreateProfile(ctx, u.ID, "b", "blue", true)

	save := func(profile int64, pr Progress, at int64) {
		t.Helper()
		if err := s.SaveProgress(ctx, profile, pr); err != nil {
			t.Fatal(err)
		}
		// pin updated_at so ordering does not depend on the clock
		if _, err := conn.Exec("UPDATE progress SET updated_at = $1 WHERE profile_id = $2 AND tmdb_id = $3 AND season = $4 AND episode = $5",
			at, profile, pr.ID, pr.Season, pr.Episode); err != nil {
			t.Fatal(err)
		}
	}
	save(p.ID, Progress{Type: "movie", ID: 1, Position: 10, Duration: 100, Source: "src", Hoster: "h", Lang: "fr"}, 100) // in progress
	save(p.ID, Progress{Type: "movie", ID: 2, Position: 99, Duration: 100}, 200)                                         // finished
	save(p.ID, Progress{Type: "tv", ID: 3, Season: 1, Episode: 1, Position: 99, Duration: 100}, 300)                     // finished episode...
	save(p.ID, Progress{Type: "tv", ID: 3, Season: 1, Episode: 2, Position: 5, Duration: 100}, 400)                      // ...then started the next
	save(other.ID, Progress{Type: "movie", ID: 4, Position: 10, Duration: 100}, 500)                                     // other profile
	save(p.ID, Progress{Type: "tv", ID: 5, Season: 1, Episode: 1, Position: 5, Duration: 100}, 50)                       // two episodes
	save(p.ID, Progress{Type: "tv", ID: 5, Season: 1, Episode: 2, Position: 5, Duration: 100}, 50)                       // saved in the same second

	got, err := s.ContinueWatching(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != 3 || got[0].Episode != 2 || got[1].ID != 1 || got[2].ID != 5 || got[2].Episode != 2 ||
		got[1].Source != "src" || got[1].Hoster != "h" || got[1].Lang != "fr" { // resuming starts on the same link
		t.Fatalf("ContinueWatching = %+v", got)
	}

	// Finished titles count too, one entry per title.
	all, err := s.RecentlyWatched(ctx, p.ID, "", 10)
	if err != nil || len(all) != 4 || all[0].ID != 3 || all[1].ID != 2 || all[2].ID != 1 || all[3].ID != 5 {
		t.Fatalf("RecentlyWatched = %+v, %v", all, err)
	}
	movies, _ := s.RecentlyWatched(ctx, p.ID, "movie", 1)
	if len(movies) != 1 || movies[0].ID != 2 {
		t.Fatalf("RecentlyWatched(movie, 1) = %+v", movies)
	}
}

// Stats group by the year of the last playback, cap the time at the duration and pick the most watched show.
func TestWatchStats(t *testing.T) {
	conn := testDB(t)
	s, ctx := New(conn), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	p, _ := s.CreateProfile(ctx, u.ID, "a", "red", true)
	const y2024, y2025 = 1717200000, 1748736000 // June 1st, far from any time zone edge
	for _, pr := range []struct {
		Progress
		at int64
	}{
		{Progress{Type: "movie", ID: 1, Position: 120, Duration: 100}, y2025}, // position past the duration
		{Progress{Type: "tv", ID: 2, Title: "Small", Season: 1, Episode: 1, Position: 50, Duration: 100}, y2025},
		{Progress{Type: "tv", ID: 3, Title: "Big", Season: 1, Episode: 1, Position: 100, Duration: 100}, y2025},
		{Progress{Type: "tv", ID: 3, Title: "Big", Season: 1, Episode: 2, Position: 100, Duration: 100}, y2025},
		{Progress{Type: "movie", ID: 4, Position: 30, Duration: 100}, y2024},
	} {
		if err := s.SaveProgress(ctx, p.ID, pr.Progress); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec("UPDATE progress SET updated_at = $1 WHERE tmdb_id = $2 AND episode = $3", pr.at, pr.ID, pr.Episode); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.WatchStats(ctx, p.ID)
	want := []YearStats{
		{Year: 2025, MovieSeconds: 100, ShowSeconds: 250, Movies: 1, Episodes: 3, TopShow: "Big"},
		{Year: 2024, MovieSeconds: 30, Movies: 1},
	}
	if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("WatchStats = %+v, %v", got, err)
	}
	if h, _ := s.History(ctx, p.ID, 10); len(h) != 5 || h[4].ID != 4 || h[4].WatchedAt != y2024 {
		t.Fatalf("History = %+v", h)
	}
}

// Marking as watched finishes what is not, and leaves the date of what already is (watch time stats per year).
func TestMarkWatched(t *testing.T) {
	conn := testDB(t)
	s, ctx := New(conn), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	p, _ := s.CreateProfile(ctx, u.ID, "a", "red", true)
	ep := func(n int, pos float64) Progress {
		return Progress{Type: "tv", ID: 1, Season: 1, Episode: n, Position: pos, Duration: 100}
	}
	_ = s.SaveProgress(ctx, p.ID, ep(1, 99)) // finished long ago
	_ = s.SaveProgress(ctx, p.ID, ep(2, 10)) // started
	if _, err := conn.Exec("UPDATE progress SET updated_at = 1"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkWatched(ctx, p.ID, []Progress{ep(1, 0), ep(2, 0), ep(3, 0)}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.TitleProgress(ctx, p.ID, "tv", 1)
	at := map[int]Progress{}
	for _, g := range got {
		at[g.Episode] = g
	}
	if len(got) != 3 || at[1].UpdatedAt != 1 || at[1].Position != 99 || at[2].UpdatedAt == 1 || at[2].Position != 100 || at[3].Position != 100 {
		t.Fatalf("TitleProgress = %+v", got)
	}
}

// A thumbs down hides a family list title from that profile only.
func TestFamilyListThumbsDown(t *testing.T) {
	s, ctx := New(testDB(t)), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	toto, _ := s.CreateProfile(ctx, u.ID, "toto", "red", true)
	other, _ := s.CreateProfile(ctx, u.ID, "other", "blue", true)
	_ = s.AddToFamilyList(ctx, u.ID, ListItem{Type: "movie", ID: 1})
	_ = s.AddToFamilyList(ctx, u.ID, ListItem{Type: "movie", ID: 2})
	if err := s.Rate(ctx, toto.ID, RatedItem{ListItem: ListItem{Type: "movie", ID: 1}, Rating: -1}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.FamilyList(ctx, u.ID, toto.ID); len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("toto = %+v", got)
	}
	if got, _ := s.FamilyList(ctx, u.ID, other.ID); len(got) != 2 {
		t.Fatalf("other = %+v", got)
	}
	_ = s.Rate(ctx, toto.ID, RatedItem{ListItem: ListItem{Type: "movie", ID: 1}, Rating: 0})
	if got, _ := s.FamilyList(ctx, u.ID, toto.ID); len(got) != 2 {
		t.Fatalf("toto after removing the thumb = %+v", got)
	}
}

// Finished: movies watched to the end, shows watched up to their latest aired episode, until a new one airs.
func TestFinished(t *testing.T) {
	s, ctx := New(testDB(t)), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	p, _ := s.CreateProfile(ctx, u.ID, "a", "red", true)
	now := time.Now()
	_ = s.SaveShow(ctx, 10, tmdb.Airing{Status: "Returning Series", LastSeason: 1, LastEpisode: 2}, now)
	_ = s.MarkWatched(ctx, p.ID, []Progress{
		{Type: "movie", ID: 1, Duration: 100},
		{Type: "tv", ID: 10, Season: 1, Episode: 2, Duration: 100},
	})
	_ = s.SaveProgress(ctx, p.ID, Progress{Type: "movie", ID: 2, Position: 10, Duration: 100}) // started only
	finished := func() map[int]bool {
		t.Helper()
		got, err := s.Finished(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		ids := map[int]bool{}
		for _, g := range got {
			ids[g.ID] = true
		}
		return ids
	}
	if ids := finished(); len(ids) != 2 || !ids[1] || !ids[10] {
		t.Fatalf("Finished = %v", ids)
	}
	_ = s.SaveShow(ctx, 10, tmdb.Airing{Status: "Returning Series", LastSeason: 1, LastEpisode: 3}, now) // new episode
	if ids := finished(); len(ids) != 1 || !ids[1] {
		t.Fatalf("Finished after a new episode = %v", ids)
	}
}

// The stream limit must reach the session lookup the play handler reads it from.
func TestStreamLimit(t *testing.T) {
	s, ctx := New(testDB(t)), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	_ = s.CreateSession(ctx, "tok", u.ID, time.Now().Add(time.Hour))
	if err := s.SetStreamLimit(ctx, u.ID, 20); err != nil {
		t.Fatal(err)
	}
	if got, err := s.UserBySession(ctx, "tok"); err != nil || got.MaxStreamMbps != 20 {
		t.Fatalf("UserBySession = %+v, %v", got, err)
	}
	if err := s.SetStreamLimit(ctx, u.ID+1, 20); err != ErrNotFound {
		t.Fatalf("unknown user: %v", err)
	}
}

func TestCreateSessionDropsExpired(t *testing.T) {
	conn := testDB(t)
	s, ctx := New(conn), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	_ = s.CreateSession(ctx, "old", u.ID, time.Now().Add(-time.Hour))
	if err := s.CreateSession(ctx, "new", u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d sessions, %v; want only the new one", n, err)
	}
}

func TestSetPasswordLogsOut(t *testing.T) {
	conn := testDB(t)
	s, ctx := New(conn), context.Background()
	u, _ := s.CreateUser(ctx, "a", "old", false)
	_ = s.CreateSession(ctx, "tok", u.ID, time.Now().Add(time.Hour))
	if err := s.SetPassword(ctx, u.ID, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, "tok"); err != ErrNotFound {
		t.Fatalf("session still valid: %v", err)
	}
	if got, _ := s.UserByID(ctx, u.ID); got.PasswordHash != "new" {
		t.Fatalf("hash = %q", got.PasswordHash)
	}
}

func TestCreateUserConflict(t *testing.T) {
	conn := testDB(t)
	s, ctx := New(conn), context.Background()
	if _, err := s.CreateUser(ctx, "Alice", "h", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "alice", "h", false); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}
