package api

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/ermos/kinora/internal/store"
)

// @Summary  "My list" of the profile
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.ListItem
// @Router   /library/list [get]
func (h *Handler) myList(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.MyList(r.Context(), currentProfile(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// @Summary  Add a title to "My list"
// @Tags     library
// @Security ProfileHeader
// @Param    body          body    store.ListItem  true  "Title"
// @Success  204
// @Router   /library/list [put]
func (h *Handler) addToList(w http.ResponseWriter, r *http.Request) {
	var it store.ListItem
	if !readJSON(w, r, &it) {
		return
	}
	if (it.Type != "movie" && it.Type != "tv") || it.ID <= 0 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	if err := h.store.AddToList(r.Context(), currentProfile(r), it); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Remove a title from "My list"
// @Tags     library
// @Security ProfileHeader
// @Param    type          path    string  true  "movie or tv"
// @Param    id            path    int     true  "TMDB ID"
// @Success  204
// @Router   /library/list/{type}/{id} [delete]
func (h *Handler) removeFromList(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	id, ok2 := pathInt(r, "id")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	if err := h.store.RemoveFromList(r.Context(), currentProfile(r), kind, id); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary      "Continue watching" row of the profile
// @Description  Titles in progress and the next episode of the shows whose last watched episode is finished, most
// @Description  recent first. A next episode the profile caught up with (badge) goes first if it aired in the last
// @Description  month, last otherwise.
// @Tags         library
// @Security     ProfileHeader
// @Success      200  {array}  store.Progress
// @Router       /library/progress [get]
func (h *Handler) continueWatching(w http.ResponseWriter, r *http.Request) {
	ps, err := h.store.ContinueWatching(r.Context(), currentProfile(r))
	if err != nil {
		internalError(w, err)
		return
	}
	news, err := h.store.NextEpisodes(r.Context(), currentProfile(r), time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	started := map[int]bool{}
	for _, p := range ps {
		if p.Type == "tv" {
			started[p.ID] = true
		}
	}
	recent, old := []store.Progress{}, []store.Progress{}
	for _, n := range news {
		if started[n.ID] { // rewatching an older episode: that one stays the card
			continue
		}
		switch {
		case n.Badge == "":
			ps = append(ps, n.Progress)
		case n.Recent:
			recent = append(recent, n.Progress)
		default:
			old = append(old, n.Progress)
		}
	}
	slices.SortStableFunc(ps, func(a, b store.Progress) int { return cmp.Compare(b.UpdatedAt, a.UpdatedAt) })
	writeJSON(w, http.StatusOK, append(append(recent, ps...), old...))
}

// RefreshShows updates the airing state of the watched shows that are due (see store.ShowsToCheck). Run in the
// background: TMDB has no batch endpoint, but only a few shows are due at a time.
func (h *Handler) RefreshShows(ctx context.Context) {
	now := time.Now()
	ids, err := h.store.ShowsToCheck(ctx, now)
	if err != nil {
		slog.Warn("shows to check", "err", err)
		return
	}
	for _, id := range ids {
		a, err := h.tmdb.Airing(ctx, id)
		if err == nil {
			err = h.store.SaveShow(ctx, id, a, now)
		}
		if err != nil {
			slog.Warn("show refresh failed", "id", id, "err", err)
		}
	}
	if len(ids) > 0 {
		slog.Info("shows refreshed", "count", len(ids))
	}
}

// @Summary  Every title the profile played, finished or not, most recent first
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.Watched
// @Router   /library/watched [get]
func (h *Handler) watched(w http.ResponseWriter, r *http.Request) {
	ws, err := h.store.RecentlyWatched(r.Context(), currentProfile(r), "", 1000)
	if err != nil {
		internalError(w, err)
		return
	}
	if ws == nil {
		ws = []store.Watched{}
	}
	writeJSON(w, http.StatusOK, ws)
}

// @Summary  Watch progress of a title (every episode for a show), most recent first
// @Tags     library
// @Security ProfileHeader
// @Param    type          path    string  true  "movie or tv"
// @Param    id            path    int     true  "TMDB ID"
// @Success  200  {array}  store.Progress
// @Router   /library/progress/{type}/{id} [get]
func (h *Handler) titleProgress(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	id, ok2 := pathInt(r, "id")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	ps, err := h.store.TitleProgress(r.Context(), currentProfile(r), kind, id)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

// @Summary  Save the playback position
// @Tags     library
// @Security ProfileHeader
// @Param    body          body    store.Progress  true  "Position"
// @Success  204
// @Router   /library/progress [put]
func (h *Handler) saveProgress(w http.ResponseWriter, r *http.Request) {
	var p store.Progress
	if !readJSON(w, r, &p) {
		return
	}
	if (p.Type != "movie" && p.Type != "tv") || p.ID <= 0 || p.Duration <= 0 || p.Position < 0 {
		writeError(w, http.StatusBadRequest, errInvalidProgress)
		return
	}
	if err := h.store.SaveProgress(r.Context(), currentProfile(r), p); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
