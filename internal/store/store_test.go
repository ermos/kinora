package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ermos/kinora/internal/db"
)

// testDB opens TEST_DATABASE_URL (skips without it) in a schema of its own, dropped after the test.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	conn, err := db.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		_, _ = admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})
	return conn
}

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
	save(p.ID, Progress{Type: "movie", ID: 1, Position: 10, Duration: 100}, 100)                     // in progress
	save(p.ID, Progress{Type: "movie", ID: 2, Position: 99, Duration: 100}, 200)                     // finished
	save(p.ID, Progress{Type: "tv", ID: 3, Season: 1, Episode: 1, Position: 99, Duration: 100}, 300) // finished episode...
	save(p.ID, Progress{Type: "tv", ID: 3, Season: 1, Episode: 2, Position: 5, Duration: 100}, 400)  // ...then started the next
	save(other.ID, Progress{Type: "movie", ID: 4, Position: 10, Duration: 100}, 500)                 // other profile
	save(p.ID, Progress{Type: "tv", ID: 5, Season: 1, Episode: 1, Position: 5, Duration: 100}, 50)   // two episodes
	save(p.ID, Progress{Type: "tv", ID: 5, Season: 1, Episode: 2, Position: 5, Duration: 100}, 50)   // saved in the same second

	got, err := s.ContinueWatching(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != 3 || got[0].Episode != 2 || got[1].ID != 1 || got[2].ID != 5 || got[2].Episode != 2 {
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
