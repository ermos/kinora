package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ermos/kinora/internal/aniskip"
	"github.com/ermos/kinora/internal/scraper"
	"github.com/ermos/kinora/internal/store"
	"github.com/ermos/kinora/internal/stream"
	"github.com/ermos/kinora/internal/tmdb"
)

const (
	sessionCookie = "kinora_session"
	sessionTTL    = 30 * 24 * time.Hour
	profileHeader = "X-Profile-ID"
	maxProfiles   = 5
)

type Handler struct {
	store     *store.Store
	tmdb      *tmdb.Client
	signer    *stream.Signer
	proxy     *stream.Proxy
	linkCache linkCache
	lang      atomic.Pointer[scraper.Language] // instance language, chosen at setup
	aniskip   *aniskip.Client
}

func New(st *store.Store, tm *tmdb.Client, signer *stream.Signer, lang scraper.Language) *Handler {
	h := &Handler{store: st, tmdb: tm, signer: signer, proxy: stream.NewProxy(signer, "/api/v1/proxy"), aniskip: aniskip.New()}
	h.lang.Store(&lang)
	return h
}

func (h *Handler) language() scraper.Language { return *h.lang.Load() }

// Routes returns the /api/v1 handler.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	public := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, fn) }
	user := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, h.requireUser(fn)) }
	profile := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, h.requireUser(h.requireProfile(fn))) }
	admin := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, h.requireUser(h.requireAdmin(fn))) }

	public("GET /api/v1/instance", h.instance)
	public("POST /api/v1/setup", h.setup)
	public("POST /api/v1/auth/login", h.login)
	public("POST /api/v1/auth/logout", h.logout)

	user("GET /api/v1/me", h.me)
	user("PUT /api/v1/me/password", h.changePassword)
	user("GET /api/v1/profiles", h.listProfiles)
	user("POST /api/v1/profiles", h.createProfile)
	user("PATCH /api/v1/profiles/{id}", h.updateProfile)
	user("DELETE /api/v1/profiles/{id}", h.deleteProfile)

	user("GET /api/v1/catalog/home", h.home)
	user("GET /api/v1/catalog/genres", h.genres)
	user("GET /api/v1/catalog/discover", h.discover)
	user("GET /api/v1/catalog/search", h.search)
	user("GET /api/v1/titles/{type}/{id}", h.title)
	user("GET /api/v1/titles/tv/{id}/seasons/{season}", h.season)
	user("GET /api/v1/titles/{type}/{id}/links", h.links)
	user("POST /api/v1/play", h.play)
	public("GET /api/v1/proxy", h.proxy.ServeHTTP) // the signed, expiring URL is the credential

	profile("GET /api/v1/titles/{type}/{id}/segments", h.segments)
	profile("GET /api/v1/catalog/foryou", h.forYou)
	profile("GET /api/v1/library/list", h.myList)
	profile("PUT /api/v1/library/list", h.addToList)
	profile("DELETE /api/v1/library/list/{type}/{id}", h.removeFromList)
	profile("GET /api/v1/library/progress", h.continueWatching)
	profile("GET /api/v1/library/progress/{type}/{id}", h.titleProgress)
	profile("PUT /api/v1/library/progress", h.saveProgress)

	admin("GET /api/v1/admin/users", h.listUsers)
	admin("POST /api/v1/admin/users", h.createUser)
	admin("DELETE /api/v1/admin/users/{id}", h.deleteUser)
	admin("PUT /api/v1/admin/instance", h.updateInstance)
	admin("GET /api/v1/admin/flaresolverr", h.getFlareSolverr)
	admin("PUT /api/v1/admin/flaresolverr", h.updateFlareSolverr)

	return csrf(mux)
}

// csrf rejects state-changing requests that are not JSON: browsers cannot send a cross-site
// application/json request without a CORS preflight we never answer, so with SameSite=Lax cookies this
// is enough.
func csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, errUnsupportedMedia)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type ctxKey int

const (
	userKey ctxKey = iota
	profileKey
)

func currentUser(r *http.Request) store.User { return r.Context().Value(userKey).(store.User) }
func currentProfile(r *http.Request) int64   { return r.Context().Value(profileKey).(int64) }

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

func (h *Handler) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, errNotLoggedIn)
			return
		}
		u, err := h.store.UserBySession(r.Context(), hashToken(c.Value))
		if err != nil {
			writeError(w, http.StatusUnauthorized, errSessionExpired)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin {
			writeError(w, http.StatusForbidden, errAdminOnly)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireProfile checks the X-Profile-ID header belongs to the logged-in account.
func (h *Handler) requireProfile(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.Header.Get(profileHeader), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, errMissingProfile)
			return
		}
		owner, err := h.store.ProfileOwner(r.Context(), id)
		if err != nil || owner != currentUser(r).ID {
			writeError(w, http.StatusForbidden, errUnknownProfile)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), profileKey, id)))
	})
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expires := time.Now().Add(sessionTTL)
	if err := h.store.CreateSession(r.Context(), hashToken(token), userID, expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func internalError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, errInternal)
}

func storeError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	internalError(w, err)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return false
	}
	return true
}

func pathInt(r *http.Request, name string) (int, bool) {
	n, err := strconv.Atoi(r.PathValue(name))
	return n, err == nil && n > 0
}

func mediaType(r *http.Request) (string, bool) {
	t := r.PathValue("type")
	if t == "" {
		t = r.URL.Query().Get("type")
	}
	return t, t == "movie" || t == "tv"
}
