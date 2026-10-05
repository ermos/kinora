package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ermos/kinora/internal/db"
)

func TestContinueWatching(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
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
		if _, err := conn.Exec("UPDATE progress SET updated_at = ? WHERE profile_id = ? AND tmdb_id = ? AND season = ? AND episode = ?",
			at, profile, pr.ID, pr.Season, pr.Episode); err != nil {
			t.Fatal(err)
		}
	}
	save(p.ID, Progress{Type: "movie", ID: 1, Position: 10, Duration: 100}, 100)                     // in progress
	save(p.ID, Progress{Type: "movie", ID: 2, Position: 99, Duration: 100}, 200)                     // finished
	save(p.ID, Progress{Type: "tv", ID: 3, Season: 1, Episode: 1, Position: 99, Duration: 100}, 300) // finished episode...
	save(p.ID, Progress{Type: "tv", ID: 3, Season: 1, Episode: 2, Position: 5, Duration: 100}, 400)  // ...then started the next
	save(other.ID, Progress{Type: "movie", ID: 4, Position: 10, Duration: 100}, 500)                 // other profile

	got, err := s.ContinueWatching(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 3 || got[0].Episode != 2 || got[1].ID != 1 {
		t.Fatalf("ContinueWatching = %+v", got)
	}
}
