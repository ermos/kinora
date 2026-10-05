package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ermos/istream/internal/api"
	"github.com/ermos/istream/internal/config"
	"github.com/ermos/istream/internal/db"
	"github.com/ermos/istream/internal/scraper"
	"github.com/ermos/istream/internal/store"
	"github.com/ermos/istream/internal/stream"
	"github.com/ermos/istream/internal/tmdb"
	"github.com/ermos/istream/ui"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return err
	}
	conn, err := db.Open(filepath.Join(cfg.DataDir, "istream.db"))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	st := store.New(conn)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	secret, err := signingKey(ctx, st)
	if err != nil {
		return err
	}
	go func() {
		for {
			if err := syncSources(ctx, st); err != nil {
				slog.Warn("sites.json sync failed", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(6 * time.Hour):
			}
		}
	}()

	lang, _ := scraper.LanguageByCode(scraper.DefaultLanguage)
	if code, err := st.Setting(ctx, "language"); err == nil {
		if l, ok := scraper.LanguageByCode(code); ok {
			lang = l
		}
	}
	h := api.New(st, tmdb.New(cfg.TMDBKey, lang.TMDB), stream.NewSigner(secret), lang)
	mux := http.NewServeMux()
	mux.Handle("/api/", h.Routes())
	mux.Handle("/", uiHandler(cfg.UIDevURL))

	srv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("istream listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// uiHandler serves the embedded web app, or proxies to the Expo dev server (hot reload, same origin as the API).
func uiHandler(devURL string) http.Handler {
	if devURL == "" {
		return ui.Handler()
	}
	u, err := url.Parse(devURL)
	if err != nil {
		slog.Error("invalid UI_DEV_URL", "err", err)
		return ui.Handler()
	}
	slog.Info("proxying the UI to the Expo dev server", "url", devURL)
	return httputil.NewSingleHostReverseProxy(u)
}

// signingKey is generated once and kept in the database so signed links survive restarts.
func signingKey(ctx context.Context, st *store.Store) ([]byte, error) {
	v, err := st.Setting(ctx, "signing_key")
	if errors.Is(err, store.ErrNotFound) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		v = hex.EncodeToString(b)
		err = st.SetSetting(ctx, "signing_key", v)
	}
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(v)
}

func syncSources(ctx context.Context, st *store.Store) error {
	urls, err := scraper.FetchSiteURLs(ctx)
	if err != nil {
		return err
	}
	for id, u := range urls {
		if err := st.SetSyncedURL(ctx, id, u); err != nil {
			return err
		}
	}
	slog.Info("source URLs synced from vStream", "count", len(urls))
	return nil
}
