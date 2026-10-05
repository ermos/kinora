package config

import (
	"errors"
	"os"
)

type Config struct {
	Addr    string
	DataDir string
	TMDBKey string
	// UIDevURL proxies the UI to an Expo dev server (make dev) instead of serving the embedded export.
	UIDevURL string
}

func Load() (Config, error) {
	c := Config{
		Addr:     env("ADDR", ":8080"),
		DataDir:  env("DATA_DIR", "./data"),
		TMDBKey:  os.Getenv("TMDB_API_KEY"),
		UIDevURL: os.Getenv("UI_DEV_URL"),
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
