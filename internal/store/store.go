package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	IsAdmin      bool   `json:"isAdmin"`
	PasswordHash string `json:"-"`
	// MaxStreamMbps caps the throughput of each stream the account plays, in Mbit/s (0: unlimited).
	MaxStreamMbps int `json:"maxStreamMbps"`
}

type Profile struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	// SkipSegments shows the "skip intro" and "skip credits" buttons.
	SkipSegments bool `json:"skipSegments"`
}

type ListItem struct {
	Type   string `json:"type"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Poster string `json:"poster"`
}

type Progress struct {
	Type     string  `json:"type"`
	ID       int     `json:"id"`
	Season   int     `json:"season"`
	Episode  int     `json:"episode"`
	Title    string  `json:"title"`
	Poster   string  `json:"poster"`
	Backdrop string  `json:"backdrop"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	// Badge marks a show's next episode that aired since the profile caught up (duration 0, not started).
	Badge string `json:"badge,omitempty" enums:"newEpisode,newSeason" validate:"optional"`
	// UpdatedAt orders the "Continue watching" row.
	UpdatedAt int64 `json:"-"`
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// --- users & sessions

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, hash string, admin bool) (User, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, "INSERT INTO users (username, password_hash, is_admin) VALUES ($1, $2, $3) RETURNING id", username, hash, admin).Scan(&id)
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Username: username, IsAdmin: admin, PasswordHash: hash}, nil
}

func (s *Store) scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.PasswordHash, &u.MaxStreamMbps)
	return u, notFound(err)
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx, "SELECT id, username, is_admin, password_hash, max_stream_mbps FROM users WHERE lower(username) = lower($1)", username))
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx, "SELECT id, username, is_admin, password_hash, max_stream_mbps FROM users WHERE id = $1", id))
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, username, is_admin, max_stream_mbps FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.MaxStreamMbps); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

func (s *Store) SetStreamLimit(ctx context.Context, id int64, mbps int) error {
	res, err := s.db.ExecContext(ctx, "UPDATE users SET max_stream_mbps = $1 WHERE id = $2", mbps, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetPassword(ctx context.Context, id int64, hash string) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2", hash, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = $1", id)
	return err
}

func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)", tokenHash, userID, expires.Unix())
	return err
}

func (s *Store) UserBySession(ctx context.Context, tokenHash string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.is_admin, u.password_hash, u.max_stream_mbps
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = $1 AND s.expires_at > extract(epoch FROM now())::bigint`, tokenHash))
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = $1 OR expires_at <= extract(epoch FROM now())::bigint", tokenHash)
	return err
}

// --- profiles

func (s *Store) ListProfiles(ctx context.Context, userID int64) ([]Profile, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, avatar, skip_segments FROM profiles WHERE user_id = $1 ORDER BY id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.ID, &p.Name, &p.Avatar, &p.SkipSegments); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateProfile(ctx context.Context, userID int64, name, avatar string, skipSegments bool) (Profile, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, "INSERT INTO profiles (user_id, name, avatar, skip_segments) VALUES ($1, $2, $3, $4) RETURNING id", userID, name, avatar, skipSegments).Scan(&id)
	if err != nil {
		return Profile{}, err
	}
	return Profile{ID: id, Name: name, Avatar: avatar, SkipSegments: skipSegments}, nil
}

func (s *Store) UpdateProfile(ctx context.Context, userID, id int64, name, avatar string, skipSegments bool) error {
	return s.affectOne(s.db.ExecContext(ctx, "UPDATE profiles SET name = $1, avatar = $2, skip_segments = $3 WHERE id = $4 AND user_id = $5", name, avatar, skipSegments, id, userID))
}

func (s *Store) ProfileSkipSegments(ctx context.Context, id int64) (bool, error) {
	var on bool
	err := s.db.QueryRowContext(ctx, "SELECT skip_segments FROM profiles WHERE id = $1", id).Scan(&on)
	return on, err
}

func (s *Store) DeleteProfile(ctx context.Context, userID, id int64) error {
	return s.affectOne(s.db.ExecContext(ctx, "DELETE FROM profiles WHERE id = $1 AND user_id = $2", id, userID))
}

func (s *Store) ProfileOwner(ctx context.Context, id int64) (int64, error) {
	var uid int64
	err := s.db.QueryRowContext(ctx, "SELECT user_id FROM profiles WHERE id = $1", id).Scan(&uid)
	return uid, notFound(err)
}

func (s *Store) affectOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- my list

func (s *Store) MyList(ctx context.Context, profileID int64) ([]ListItem, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT media_type, tmdb_id, title, poster FROM my_list WHERE profile_id = $1 ORDER BY added_at DESC", profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListItem{}
	for rows.Next() {
		var it ListItem
		if err := rows.Scan(&it.Type, &it.ID, &it.Title, &it.Poster); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) AddToList(ctx context.Context, profileID int64, it ListItem) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO my_list (profile_id, media_type, tmdb_id, title, poster) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`, profileID, it.Type, it.ID, it.Title, it.Poster)
	return err
}

func (s *Store) RemoveFromList(ctx context.Context, profileID int64, kind string, id int) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM my_list WHERE profile_id = $1 AND media_type = $2 AND tmdb_id = $3", profileID, kind, id)
	return err
}

// --- progress

func (s *Store) SaveProgress(ctx context.Context, profileID int64, p Progress) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO progress (profile_id, media_type, tmdb_id, season, episode, title, poster, backdrop, position, duration, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, extract(epoch FROM now())::bigint)
		ON CONFLICT (profile_id, media_type, tmdb_id, season, episode) DO UPDATE SET
		position = excluded.position, duration = excluded.duration, updated_at = excluded.updated_at`,
		profileID, p.Type, p.ID, p.Season, p.Episode, p.Title, p.Poster, p.Backdrop, p.Position, p.Duration)
	return err
}

const progressCols = "media_type, tmdb_id, season, episode, title, poster, backdrop, position, duration, updated_at"

func scanProgress(rows *sql.Rows) ([]Progress, error) {
	defer rows.Close()
	out := []Progress{}
	for rows.Next() {
		var p Progress
		if err := rows.Scan(&p.Type, &p.ID, &p.Season, &p.Episode, &p.Title, &p.Poster, &p.Backdrop, &p.Position, &p.Duration, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ContinueWatching returns, per title, the last thing watched if it is not finished. DISTINCT ON keeps exactly one
// row per title, even when two episodes were saved in the same second (the furthest one wins).
func (s *Store) ContinueWatching(ctx context.Context, profileID int64) ([]Progress, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+progressCols+` FROM (
			SELECT DISTINCT ON (media_type, tmdb_id) `+progressCols+` FROM progress WHERE profile_id = $1
			ORDER BY media_type, tmdb_id, updated_at DESC, season DESC, episode DESC
		) p WHERE position < duration * 0.95 ORDER BY updated_at DESC LIMIT 20`, profileID)
	if err != nil {
		return nil, err
	}
	return scanProgress(rows)
}

// Watched is a title the profile played, finished or not.
type Watched struct {
	Type  string `json:"type"`
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// RecentlyWatched lists the titles the profile played (kind "" for both), most recent first, one per title.
func (s *Store) RecentlyWatched(ctx context.Context, profileID int64, kind string, limit int) ([]Watched, error) {
	// DISTINCT ON keeps the latest row of each title, the outer query orders titles by it.
	rows, err := s.db.QueryContext(ctx, `SELECT media_type, tmdb_id, title, updated_at FROM (
			SELECT DISTINCT ON (media_type, tmdb_id) media_type, tmdb_id, title, updated_at FROM progress
			WHERE profile_id = $1 AND ($2 = '' OR media_type = $2)
			ORDER BY media_type, tmdb_id, updated_at DESC
		) t ORDER BY updated_at DESC LIMIT $3`, profileID, kind, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Watched
	for rows.Next() {
		var w Watched
		var last int64
		if err := rows.Scan(&w.Type, &w.ID, &w.Title, &last); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// HistoryEntry is a movie or an episode the profile played.
type HistoryEntry struct {
	Type    string `json:"type"`
	ID      int    `json:"id"`
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
	Title   string `json:"title"`
	Poster  string `json:"poster"`
	// Position and Duration in seconds.
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	// WatchedAt is the Unix time of the last playback.
	WatchedAt int64 `json:"watchedAt"`
}

// History lists the movies and episodes the profile played, most recent first.
func (s *Store) History(ctx context.Context, profileID int64, limit int) ([]HistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT media_type, tmdb_id, season, episode, title, poster, position, duration, updated_at
		FROM progress WHERE profile_id = $1 ORDER BY updated_at DESC LIMIT $2`, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryEntry{}
	for rows.Next() {
		var h HistoryEntry
		if err := rows.Scan(&h.Type, &h.ID, &h.Season, &h.Episode, &h.Title, &h.Poster, &h.Position, &h.Duration, &h.WatchedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// YearStats sums up what the profile watched in a year.
type YearStats struct {
	Year int `json:"year"`
	// MovieSeconds and ShowSeconds are the time spent watching, in seconds.
	MovieSeconds float64 `json:"movieSeconds"`
	ShowSeconds  float64 `json:"showSeconds"`
	Movies       int     `json:"movies"`
	Episodes     int     `json:"episodes"`
	// TopShow is the show the profile spent the most time on, "" without any.
	TopShow string `json:"topShow"`
}

// WatchStats returns the profile's stats per year, most recent first. A title counts in the year it was last played.
func (s *Store) WatchStats(ctx context.Context, profileID int64) ([]YearStats, error) {
	// ponytail: progress keeps one row per title/episode, so a rewatch counts once. A play log table if that matters.
	rows, err := s.db.QueryContext(ctx, `WITH p AS (
			SELECT extract(year FROM to_timestamp(updated_at))::int AS y, media_type, tmdb_id, title, least(position, duration) AS t
			FROM progress WHERE profile_id = $1
		), top AS (
			SELECT DISTINCT ON (y) y, title FROM (
				SELECT y, tmdb_id, max(title) AS title, sum(t) AS t FROM p WHERE media_type = 'tv' GROUP BY y, tmdb_id
			) s ORDER BY y, t DESC
		)
		SELECT p.y,
			coalesce(sum(t) FILTER (WHERE media_type = 'movie'), 0),
			coalesce(sum(t) FILTER (WHERE media_type = 'tv'), 0),
			count(*) FILTER (WHERE media_type = 'movie'),
			count(*) FILTER (WHERE media_type = 'tv'),
			coalesce(min(top.title), '')
		FROM p LEFT JOIN top ON top.y = p.y GROUP BY p.y ORDER BY p.y DESC`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []YearStats{}
	for rows.Next() {
		var y YearStats
		if err := rows.Scan(&y.Year, &y.MovieSeconds, &y.ShowSeconds, &y.Movies, &y.Episodes, &y.TopShow); err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rows.Err()
}

func (s *Store) TitleProgress(ctx context.Context, profileID int64, kind string, id int) ([]Progress, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+progressCols+` FROM progress
		WHERE profile_id = $1 AND media_type = $2 AND tmdb_id = $3 ORDER BY updated_at DESC`, profileID, kind, id)
	if err != nil {
		return nil, err
	}
	return scanProgress(rows)
}

// --- sources

// SourceURL returns the URL of a source synced from vStream's sites.json, "" before the first sync.
func (s *Store) SourceURL(ctx context.Context, id string) string {
	var u string
	_ = s.db.QueryRowContext(ctx, "SELECT url FROM sources WHERE id = $1", id).Scan(&u)
	return u
}

func (s *Store) SetSyncedURL(ctx context.Context, id, url string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sources (id, url) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET url = excluded.url`, id, url)
	return err
}

// --- settings

func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = $1", key).Scan(&v)
	return v, notFound(err)
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value)
	return err
}
