package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	_ "modernc.org/sqlite" // the SQLite file to import
)

// importTables in foreign key order, with the columns shared by both schemas. Booleans were integers in SQLite.
var importTables = []struct {
	name, cols string
	bools      []string
}{
	{"users", "id, username, password_hash, is_admin, created_at", []string{"is_admin"}},
	{"sessions", "token_hash, user_id, expires_at", nil},
	{"profiles", "id, user_id, name, avatar, skip_segments, created_at", []string{"skip_segments"}},
	{"my_list", "profile_id, media_type, tmdb_id, title, poster, added_at", nil},
	{"progress", "profile_id, media_type, tmdb_id, season, episode, title, poster, backdrop, position, duration, updated_at", nil},
	{"sources", "id, url", nil},
	{"settings", "key, value", nil},
}

// ImportSQLite copies the database of a kinora that ran on SQLite into the (empty) PostgreSQL one, in one transaction.
func ImportSQLite(ctx context.Context, pg *sql.DB, path string) error {
	src, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	var users int
	if err := pg.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&users); err != nil {
		return err
	}
	if users > 0 {
		return fmt.Errorf("the PostgreSQL database already has %d users, import only into a fresh one", users)
	}

	tx, err := pg.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, t := range importTables {
		n, err := copyTable(ctx, src, tx, t.name, t.cols, t.bools)
		if err != nil {
			return fmt.Errorf("%s: %w", t.name, err)
		}
		slog.Info("imported", "table", t.name, "rows", n)
	}
	// Ids were copied as is: move the sequences past them.
	for _, t := range []string{"users", "profiles"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE(MAX(id), 0) + 1, false) FROM %[1]s", t)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func copyTable(ctx context.Context, src *sql.DB, tx *sql.Tx, table, cols string, bools []string) (int, error) {
	names := strings.Split(cols, ", ")
	rows, err := src.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s", cols, table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	marks := make([]string, len(names))
	for i, c := range names {
		marks[i] = fmt.Sprintf("$%d", i+1)
		for _, b := range bools {
			if c == b {
				marks[i] += " <> 0"
			}
		}
	}
	insert := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, cols, strings.Join(marks, ", "))
	n := 0
	for rows.Next() {
		vals := make([]any, len(names))
		ptrs := make([]any, len(names))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return n, err
		}
		if _, err := tx.ExecContext(ctx, insert, vals...); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}
