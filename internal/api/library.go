package api

import (
	"net/http"

	"github.com/ermos/istream/internal/store"
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

// @Summary  "Continue watching" row of the profile
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.Progress
// @Router   /library/progress [get]
func (h *Handler) continueWatching(w http.ResponseWriter, r *http.Request) {
	ps, err := h.store.ContinueWatching(r.Context(), currentProfile(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
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
