package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/ermos/kinora/internal/tmdb"
)

// ShowsToCheck lists the watched shows whose airing state is due for a refresh: never checked, an announced episode
// has aired, or not checked for a week (a month for ended shows, they rarely come back).
func (s *Store) ShowsToCheck(ctx context.Context, now time.Time) ([]int, error) {
	return queryAll(ctx, s.db, func(rows *sql.Rows, id *int) error { return rows.Scan(id) },
		`SELECT DISTINCT p.tmdb_id FROM progress p LEFT JOIN shows s ON s.tmdb_id = p.tmdb_id
		WHERE p.media_type = 'tv' AND (s.tmdb_id IS NULL
			OR (s.next_air_date <= $2::date AND s.checked_at < $1 - 6 * 3600)
			OR s.checked_at < $1 - CASE WHEN s.status IN ('Ended', 'Canceled') THEN 30 ELSE 7 END * 86400)`,
		now.Unix(), now.Format(time.DateOnly))
}

func (s *Store) SaveShow(ctx context.Context, id int, a tmdb.Airing, now time.Time) error {
	episodes := make([]int32, len(a.Episodes))
	for i, n := range a.Episodes {
		episodes[i] = int32(n) //nolint:gosec // episode counts
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO shows (tmdb_id, status, last_season, last_episode, last_air_date, next_air_date, episodes, checked_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::date, NULLIF($6, '')::date, $7, $8)
		ON CONFLICT (tmdb_id) DO UPDATE SET status = excluded.status, last_season = excluded.last_season,
			last_episode = excluded.last_episode, last_air_date = excluded.last_air_date,
			next_air_date = excluded.next_air_date, episodes = excluded.episodes, checked_at = excluded.checked_at`,
		id, a.Status, a.LastSeason, a.LastEpisode, a.LastAirDate, a.NextAirDate, episodes, now.Unix())
	return err
}

// NextEpisode is the next episode of a show the profile caught up with, aired since.
type NextEpisode struct {
	Progress
	// Recent: aired in the last month.
	Recent bool
}

// NextEpisodes finds, for each show whose furthest watched episode is finished, the next one if it has aired. It
// gets a badge when it belongs to the latest season: a new episode of the season being followed, or the premiere of
// a new one. Without a badge it is just "up next" in an older season. Most recently aired first.
func (s *Store) NextEpisodes(ctx context.Context, profileID int64, now time.Time) ([]NextEpisode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.tmdb_id, p.season, p.episode, p.title, p.poster, p.backdrop, p.updated_at,
			s.last_season, s.last_episode, COALESCE(s.episodes[p.season + 1], 0),
			COALESCE(s.last_air_date >= $2::date - 30, false)
		FROM (SELECT DISTINCT ON (tmdb_id) tmdb_id, season, episode, title, poster, backdrop, position, duration, updated_at FROM progress
			WHERE profile_id = $1 AND media_type = 'tv' ORDER BY tmdb_id, season DESC, episode DESC) p
		JOIN shows s ON s.tmdb_id = p.tmdb_id
		WHERE p.position >= p.duration * 0.95
		ORDER BY s.last_air_date DESC NULLS LAST`, profileID, now.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NextEpisode{}
	for rows.Next() {
		var n NextEpisode
		var lastSeason, lastEpisode, seasonEpisodes int
		if err := rows.Scan(&n.ID, &n.Season, &n.Episode, &n.Title, &n.Poster, &n.Backdrop, &n.UpdatedAt, &lastSeason, &lastEpisode, &seasonEpisodes, &n.Recent); err != nil {
			return nil, err
		}
		n.Type = "tv"
		if n.Episode < seasonEpisodes {
			n.Episode++
		} else {
			n.Season, n.Episode = n.Season+1, 1
		}
		if n.Season > lastSeason || n.Season == lastSeason && n.Episode > lastEpisode {
			continue // not aired yet
		}
		if n.Season == lastSeason {
			n.Badge = "newEpisode"
			if n.Episode == 1 {
				n.Badge = "newSeason"
			}
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
