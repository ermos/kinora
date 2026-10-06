package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects to PostgreSQL (a postgres:// URL) and applies pending migrations.
func Open(ctx context.Context, url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to the database: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ponytail: numbered files tracked in schema_migrations, add goose if we ever need down migrations.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)"); err != nil {
		return err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for i, f := range files {
		if err := apply(ctx, db, i+1, f.Name()); err != nil {
			return fmt.Errorf("migration %s: %w", f.Name(), err)
		}
	}
	return nil
}

// apply runs one migration in a transaction, unless already applied. The advisory lock keeps two instances
// starting together from both running it.
func apply(ctx context.Context, db *sql.DB, version int, name string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(4210)"); err != nil {
		return err
	}
	var done bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&done); err != nil || done {
		return err
	}
	q, err := migrations.ReadFile("migrations/" + name)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(q)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit()
}
