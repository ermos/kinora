package config

import (
	"errors"
	"os"
)

type Config struct {
	Addr string
	// DatabaseURL is the PostgreSQL connection URL (postgres://user:password@host:5432/db).
	DatabaseURL string
	TMDBKey     string
	// UIDevURL proxies the UI to an Expo dev server (make dev) instead of serving the embedded export.
	UIDevURL string
	// Dev (APP_ENV=development) serves the locally built TV APK as the latest release instead of GitHub's.
	Dev bool
}

func Load() (Config, error) {
	c := Config{
		Addr:        env("ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		TMDBKey:     os.Getenv("TMDB_API_KEY"),
		UIDevURL:    os.Getenv("UI_DEV_URL"),
		Dev:         os.Getenv("APP_ENV") == "development",
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required (postgres://user:password@host:5432/kinora)")
	}
	if c.TMDBKey == "" {
		return c, errors.New("TMDB_API_KEY is required (free key: https://www.themoviedb.org/settings/api)")
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
