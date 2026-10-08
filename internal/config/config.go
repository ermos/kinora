package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
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
	// TrustedProxies are the peers whose X-Forwarded-For is believed (TRUSTED_PROXIES, comma separated CIDRs).
	// nil keeps the default: loopback and private networks.
	TrustedProxies []netip.Prefix
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
	for _, s := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return c, fmt.Errorf("TRUSTED_PROXIES: %w", err)
		}
		c.TrustedProxies = append(c.TrustedProxies, p.Masked())
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
