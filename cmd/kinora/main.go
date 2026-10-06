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
	"syscall"
	"time"

	"github.com/ermos/kinora/internal/api"
	"github.com/ermos/kinora/internal/config"
	"github.com/ermos/kinora/internal/db"
	"github.com/ermos/kinora/internal/scraper"
	_ "github.com/ermos/kinora/internal/scraper/en" // English sources
	_ "github.com/ermos/kinora/internal/scraper/fr" // French sources
	"github.com/ermos/kinora/internal/store"
	"github.com/ermos/kinora/internal/stream"
	"github.com/ermos/kinora/internal/tmdb"
	"github.com/ermos/kinora/ui"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	// kinora import-sqlite data/kinora.db: one-off copy of a database from the SQLite era.
	if len(os.Args) == 3 && os.Args[1] == "import-sqlite" {
		return db.ImportSQLite(ctx, conn, os.Args[2])
	}
	st := store.New(conn)

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

	if u, err := st.Setting(ctx, "flaresolverr"); err == nil {
		scraper.SetFlareSolverr(u)
	}

	lang, _ := scraper.LanguageByCode(scraper.DefaultLanguage)
	if code, err := st.Setting(ctx, "language"); err == nil {
		if l, ok := scraper.LanguageByCode(code); ok {
			lang = l
		}
	}
	h := api.New(st, tmdb.New(cfg.TMDBKey, lang.TMDB), stream.NewSigner(secret), lang)
	go func() {
		for {
			h.RefreshShows(ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Hour):
			}
		}
	}()
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
	slog.Info("kinora listening", "addr", cfg.Addr)
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
	slog.Info("source URLs synced", "count", len(urls))
	return nil
}
