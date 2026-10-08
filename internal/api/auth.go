package api

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ermos/kinora/internal/scraper"
	"github.com/ermos/kinora/internal/store"
)

var avatars = map[string]bool{"red": true, "blue": true, "green": true, "yellow": true, "purple": true, "pink": true, "teal": true, "orange": true}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type language struct {
	Code  string `json:"code" enums:"en,fr"`
	Label string `json:"label"`
}

type instanceInfo struct {
	// SetupNeeded is true until the first admin account exists.
	SetupNeeded bool `json:"setupNeeded"`
	// Language of the instance: catalog, UI and sources.
	Language string `json:"language" enums:"en,fr"`
	// Languages that can be picked at setup: the ones supported end to end.
	Languages []language `json:"languages"`
}

type setupRequest struct {
	credentials
	Language string `json:"language" enums:"en,fr"`
}

func validCredentials(w http.ResponseWriter, c credentials) bool {
	if strings.TrimSpace(c.Username) == "" || len(c.Username) > 64 {
		writeError(w, http.StatusBadRequest, errUsernameRequired)
		return false
	}
	if len(c.Password) < 8 || len(c.Password) > 72 { // bcrypt ignores bytes past 72
		writeError(w, http.StatusBadRequest, errPasswordLength)
		return false
	}
	return true
}

// @Summary  Public instance information: setup state and language
// @Tags     auth
// @Success  200  {object}  instanceInfo
// @Router   /instance [get]
func (h *Handler) instance(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.CountUsers(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	info := instanceInfo{SetupNeeded: n == 0, Language: h.language().Code, Languages: []language{}}
	for _, l := range scraper.SupportedLanguages() {
		info.Languages = append(info.Languages, language{Code: l.Code, Label: l.Label})
	}
	writeJSON(w, http.StatusOK, info)
}

// @Summary  Create the first admin account and pick the instance language (only while no account exists), then log in
// @Tags     auth
// @Param    body  body      setupRequest  true  "Admin credentials and language"
// @Success  201   {object}  store.User
// @Failure  409   {object}  apiError
// @Router   /setup [post]
func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if !readJSON(w, r, &req) || !validCredentials(w, req.credentials) {
		return
	}
	c := req.credentials
	lang, ok := supportedLanguage(req.Language)
	if !ok {
		writeError(w, http.StatusBadRequest, errUnsupportedLanguage)
		return
	}
	// ponytail: the count check and insert are not atomic, the setup window is a single admin on first boot.
	if n, err := h.store.CountUsers(r.Context()); err != nil || n > 0 {
		writeError(w, http.StatusConflict, errAlreadySetUp)
		return
	}
	u, err := h.createAccount(r, c, true)
	if err != nil {
		internalError(w, err)
		return
	}
	if err := h.setLanguage(r, lang); err != nil {
		internalError(w, err)
		return
	}
	if err := h.startSession(w, r, u.ID); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

// createAccount creates a user with a first profile named after them, like Netflix does.
func (h *Handler) createAccount(r *http.Request, c credentials, admin bool) (store.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		return store.User{}, err
	}
	u, err := h.store.CreateUser(r.Context(), strings.TrimSpace(c.Username), string(hash), admin)
	if err != nil {
		return store.User{}, err
	}
	_, err = h.store.CreateProfile(r.Context(), u.ID, u.Username, "red", true)
	return u, err
}

// @Summary  Log in
// @Tags     auth
// @Param    body  body      credentials  true  "Credentials"
// @Success  200   {object}  store.User
// @Failure  401   {object}  apiError
// @Failure  429   {object}  apiError  "Too many failed attempts from this IP: retry after Retry-After seconds"
// @Router   /auth/login [post]
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, h.trustedProxies)
	if d := h.loginLimiter.blocked(ip); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, errTooManyAttempts)
		return
	}
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	account := strings.ToLower(strings.TrimSpace(c.Username))
	if d := h.accountLimiter.blocked(account); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, errTooManyAttempts)
		return
	}
	u, err := h.store.UserByUsername(r.Context(), strings.TrimSpace(c.Username))
	hash := u.PasswordHash
	if err != nil {
		hash = dummyHash() // an unknown username takes as long as a wrong password: no account enumeration
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil || err != nil {
		h.loginLimiter.fail(ip)
		h.accountLimiter.fail(account)
		time.Sleep(400*time.Millisecond + rand.N(200*time.Millisecond))
		writeError(w, http.StatusUnauthorized, errInvalidCredentials)
		return
	}
	if err := h.startSession(w, r, u.ID); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// dummyHash is compared against when the username does not exist, at the same cost as real hashes.
var dummyHash = sync.OnceValue(func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("kinora-dummy-password"), bcrypt.DefaultCost)
	return string(h)
})

// @Summary  Log out
// @Tags     auth
// @Success  204
// @Router   /auth/logout [post]
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = h.store.DeleteSession(r.Context(), hashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Current account
// @Tags     auth
// @Success  200  {object}  store.User
// @Router   /me [get]
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

type passwordChange struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

// @Summary  Change the account password (logs out every session)
// @Tags     auth
// @Param    body  body  passwordChange  true  "Passwords"
// @Success  204
// @Failure  401  {object}  apiError
// @Router   /me/password [put]
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var p passwordChange
	if !readJSON(w, r, &p) {
		return
	}
	u := currentUser(r)
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(p.Current)) != nil {
		writeError(w, http.StatusUnauthorized, errWrongPassword)
		return
	}
	if !validCredentials(w, credentials{Username: u.Username, Password: p.New}) {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p.New), bcrypt.DefaultCost)
	if err != nil {
		internalError(w, err)
		return
	}
	if err := h.store.SetPassword(r.Context(), u.ID, string(hash)); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- profiles

type profileBody struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	// SkipSegments shows the "skip intro" and "skip credits" buttons.
	SkipSegments bool `json:"skipSegments"`
}

func validProfile(w http.ResponseWriter, p *profileBody) bool {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 32 {
		writeError(w, http.StatusBadRequest, errNameRequired)
		return false
	}
	if !avatars[p.Avatar] {
		writeError(w, http.StatusBadRequest, errUnknownAvatar)
		return false
	}
	return true
}

// @Summary  Profiles of the account
// @Tags     profiles
// @Success  200  {array}  store.Profile
// @Router   /profiles [get]
func (h *Handler) listProfiles(w http.ResponseWriter, r *http.Request) {
	ps, err := h.store.ListProfiles(r.Context(), currentUser(r).ID)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

// @Summary  Create a profile (5 max per account)
// @Tags     profiles
// @Param    body  body      profileBody  true  "Profile"
// @Success  201   {object}  store.Profile
// @Failure  409   {object}  apiError
// @Router   /profiles [post]
func (h *Handler) createProfile(w http.ResponseWriter, r *http.Request) {
	var p profileBody
	if !readJSON(w, r, &p) || !validProfile(w, &p) {
		return
	}
	uid := currentUser(r).ID
	existing, err := h.store.ListProfiles(r.Context(), uid)
	if err != nil {
		internalError(w, err)
		return
	}
	if len(existing) >= maxProfiles {
		writeError(w, http.StatusConflict, errProfileLimit)
		return
	}
	created, err := h.store.CreateProfile(r.Context(), uid, p.Name, p.Avatar, p.SkipSegments)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// @Summary  Rename or change the avatar of a profile
// @Tags     profiles
// @Param    id    path  int          true  "Profile ID"
// @Param    body  body  profileBody  true  "Profile"
// @Success  204
// @Router   /profiles/{id} [patch]
func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	var p profileBody
	if !ok || !readJSON(w, r, &p) || !validProfile(w, &p) {
		return
	}
	if err := h.store.UpdateProfile(r.Context(), currentUser(r).ID, int64(id), p.Name, p.Avatar, p.SkipSegments); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Delete a profile and its history
// @Tags     profiles
// @Param    id  path  int  true  "Profile ID"
// @Success  204
// @Router   /profiles/{id} [delete]
func (h *Handler) deleteProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, errInvalidID)
		return
	}
	ps, err := h.store.ListProfiles(r.Context(), currentUser(r).ID)
	if err != nil {
		internalError(w, err)
		return
	}
	if len(ps) <= 1 {
		writeError(w, http.StatusConflict, errLastProfile)
		return
	}
	if err := h.store.DeleteProfile(r.Context(), currentUser(r).ID, int64(id)); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- admin: accounts

type newUser struct {
	credentials
	IsAdmin bool `json:"isAdmin"`
}

// @Summary  List accounts
// @Tags     admin
// @Success  200  {array}  store.User
// @Router   /admin/users [get]
func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	us, err := h.store.ListUsers(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, us)
}

// @Summary  Create an account
// @Tags     admin
// @Param    body  body      newUser  true  "Account"
// @Success  201   {object}  store.User
// @Failure  409   {object}  apiError
// @Router   /admin/users [post]
func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var nu newUser
	if !readJSON(w, r, &nu) || !validCredentials(w, nu.credentials) {
		return
	}
	u, err := h.createAccount(r, nu.credentials, nu.IsAdmin)
	if errors.Is(err, store.ErrConflict) { // the unique index on lower(username) decides, even for two requests at once
		writeError(w, http.StatusConflict, errUsernameTaken)
		return
	} else if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

// @Summary  Delete an account
// @Tags     admin
// @Param    id  path  int  true  "User ID"
// @Success  204
// @Failure  409  {object}  apiError
// @Router   /admin/users/{id} [delete]
func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errInvalidID)
		return
	}
	if id == currentUser(r).ID {
		writeError(w, http.StatusConflict, errDeleteSelf)
		return
	}
	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type userUpdate struct {
	// MaxStreamMbps caps the throughput of each stream the account plays, in Mbit/s (0: unlimited).
	MaxStreamMbps int `json:"maxStreamMbps" minimum:"0" maximum:"10000"`
}

// @Summary  Update an account. A new stream limit applies from the next title played.
// @Tags     admin
// @Param    id    path  int         true  "User ID"
// @Param    body  body  userUpdate  true  "Settings"
// @Success  204
// @Failure  400  {object}  apiError
// @Failure  404  {object}  apiError
// @Router   /admin/users/{id} [patch]
func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errInvalidID)
		return
	}
	var req userUpdate
	if !readJSON(w, r, &req) {
		return
	}
	if req.MaxStreamMbps < 0 || req.MaxStreamMbps > 10000 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	if err := h.store.SetStreamLimit(r.Context(), id, req.MaxStreamMbps); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	} else if err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func supportedLanguage(code string) (scraper.Language, bool) {
	for _, l := range scraper.SupportedLanguages() {
		if l.Code == code {
			return l, true
		}
	}
	return scraper.Language{}, false
}

// setLanguage persists the instance language and switches the catalog to it. Cached links are keyed by
// language, so they need no flush.
func (h *Handler) setLanguage(r *http.Request, lang scraper.Language) error {
	if err := h.store.SetSetting(r.Context(), "language", lang.Code); err != nil {
		return err
	}
	h.lang.Store(&lang)
	h.tmdb.SetLanguage(lang.TMDB)
	return nil
}

type instanceUpdate struct {
	Language string `json:"language" enums:"en,fr"`
}

// @Summary  Change the instance language (catalog, UI and sources)
// @Tags     admin
// @Param    body  body  instanceUpdate  true  "Language"
// @Success  204
// @Failure  400  {object}  apiError
// @Router   /admin/instance [put]
func (h *Handler) updateInstance(w http.ResponseWriter, r *http.Request) {
	var req instanceUpdate
	if !readJSON(w, r, &req) {
		return
	}
	lang, ok := supportedLanguage(req.Language)
	if !ok {
		writeError(w, http.StatusBadRequest, errUnsupportedLanguage)
		return
	}
	if err := h.setLanguage(r, lang); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type flareSolverrSettings struct {
	// URL of the FlareSolverr server, "" when disabled.
	URL string `json:"url"`
}

// @Summary  FlareSolverr server used for sources behind Cloudflare
// @Tags     admin
// @Success  200  {object}  flareSolverrSettings
// @Router   /admin/flaresolverr [get]
func (h *Handler) getFlareSolverr(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, flareSolverrSettings{URL: h.FlareSolverr()})
}

// @Summary  Set the FlareSolverr server (empty URL disables it), checked before saving. Sources behind Cloudflare are queried while it is set.
// @Tags     admin
// @Param    body  body  flareSolverrSettings  true  "FlareSolverr URL"
// @Success  204
// @Failure  400  {object}  apiError
// @Router   /admin/flaresolverr [put]
func (h *Handler) updateFlareSolverr(w http.ResponseWriter, r *http.Request) {
	var req flareSolverrSettings
	if !readJSON(w, r, &req) {
		return
	}
	u := strings.TrimRight(strings.TrimSpace(req.URL), "/")
	if u != "" {
		if err := scraper.PingFlareSolverr(r.Context(), u); err != nil {
			writeError(w, http.StatusBadRequest, errFlareSolverr)
			return
		}
	}
	if err := h.store.SetSetting(r.Context(), "flaresolverr", u); err != nil {
		internalError(w, err)
		return
	}
	h.SetFlareSolverr(u)
	w.WriteHeader(http.StatusNoContent)
}
