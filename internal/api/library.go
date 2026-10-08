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
	if !validItem(it) {
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

// validProgress: a movie has no season nor episode (0), an episode of a show has both, from 1.
func validProgress(p store.Progress) bool {
	if p.ID <= 0 || p.Duration <= 0 || p.Position < 0 {
		return false
	}
	switch p.Type {
	case "movie":
		return p.Season == 0 && p.Episode == 0
	case "tv":
		return p.Season >= 1 && p.Episode >= 1
	}
	return false
}

func validItem(it store.ListItem) bool { return (it.Type == "movie" || it.Type == "tv") && it.ID > 0 }

// @Summary  "Family list", shared by every profile of the account, without the titles the profile gave a thumbs down
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.ListItem
// @Router   /library/family [get]
func (h *Handler) familyList(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.FamilyList(r.Context(), currentUser(r).ID, currentProfile(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// @Summary  Add a title to the "Family list"
// @Tags     library
// @Security SessionCookie
// @Param    body  body  store.ListItem  true  "Title"
// @Success  204
// @Router   /library/family [put]
func (h *Handler) addToFamilyList(w http.ResponseWriter, r *http.Request) {
	var it store.ListItem
	if !readJSON(w, r, &it) {
		return
	}
	if !validItem(it) {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	if err := h.store.AddToFamilyList(r.Context(), currentUser(r).ID, it); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Remove a title from the "Family list"
// @Tags     library
// @Security SessionCookie
// @Param    type  path  string  true  "movie or tv"
// @Param    id    path  int     true  "TMDB ID"
// @Success  204
// @Router   /library/family/{type}/{id} [delete]
func (h *Handler) removeFromFamilyList(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	id, ok2 := pathInt(r, "id")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	if err := h.store.RemoveFromFamilyList(r.Context(), currentUser(r).ID, kind, id); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Thumbs up and down of the profile, most recent first
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.RatedItem
// @Router   /library/ratings [get]
func (h *Handler) ratings(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Ratings(r.Context(), currentProfile(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// @Summary  Thumbs up (1) or down (-1) on a title, 0 removes it
// @Tags     library
// @Security ProfileHeader
// @Param    body  body  store.RatedItem  true  "Title and rating"
// @Success  204
// @Router   /library/ratings [put]
func (h *Handler) rate(w http.ResponseWriter, r *http.Request) {
	var it store.RatedItem
	if !readJSON(w, r, &it) {
		return
	}
	if !validItem(it.ListItem) || it.Rating < -1 || it.Rating > 1 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	if err := h.store.Rate(r.Context(), currentProfile(r), it); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Movies watched to the end and shows watched up to their latest aired episode, most recent first
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.ListItem
// @Router   /library/finished [get]
func (h *Handler) finished(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Finished(r.Context(), currentProfile(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// fallbackRuntime stands for a runtime TMDB does not know, in minutes.
// ponytail: a flat guess, skews the watch time stats a bit for those titles only.
const fallbackRuntime = 40

// @Summary      Mark a title as entirely watched
// @Description  A movie, or every episode of a show aired so far. Titles already finished keep their date.
// @Tags         library
// @Security     ProfileHeader
// @Param        type  path  string  true  "movie or tv"
// @Param        id    path  int     true  "TMDB ID"
// @Success      204
// @Router       /library/watched/{type}/{id} [post]
func (h *Handler) markWatched(w http.ResponseWriter, r *http.Request) {
	kind, ok := mediaType(r)
	id, ok2 := pathInt(r, "id")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, errInvalidRequest)
		return
	}
	ctx := r.Context()
	d, err := h.tmdb.Details(ctx, kind, id)
	if err != nil {
		internalError(w, err)
		return
	}
	base := store.Progress{Type: kind, ID: id, Title: d.Title, Poster: d.Poster, Backdrop: d.Backdrop}
	minutes := func(rt int) float64 { return float64(cmp.Or(rt, fallbackRuntime) * 60) }
	var ps []store.Progress
	if kind == "movie" {
		base.Duration = minutes(d.Runtime)
		ps = append(ps, base)
	} else {
		a, err := h.tmdb.Airing(ctx, id)
		if err == nil {
			err = h.store.SaveShow(ctx, id, a, time.Now()) // "finished" right away
		}
		if err != nil {
			internalError(w, err)
			return
		}
		for season := 1; season <= a.LastSeason; season++ {
			eps, err := h.tmdb.Season(ctx, id, season)
			if err != nil {
				internalError(w, err)
				return
			}
			for _, e := range eps {
				if season == a.LastSeason && e.Number > a.LastEpisode {
					break // not aired yet
				}
				p := base
				p.Season, p.Episode, p.Duration = season, e.Number, minutes(e.Runtime)
				ps = append(ps, p)
			}
		}
	}
	if err := h.store.MarkWatched(ctx, currentProfile(r), ps); err != nil {
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
	writeJSON(w, http.StatusOK, ws)
}

// @Summary  Time spent watching per year, most recent first
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.YearStats
// @Router   /library/stats [get]
func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	years, err := h.store.WatchStats(r.Context(), currentProfile(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, years)
}

// @Summary  Last movies and episodes the profile played, most recent first
// @Tags     library
// @Security ProfileHeader
// @Success  200  {array}  store.HistoryEntry
// @Router   /library/history [get]
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.History(r.Context(), currentProfile(r), 500)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
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
	if !validProgress(p) {
		writeError(w, http.StatusBadRequest, errInvalidProgress)
		return
	}
	if err := h.store.SaveProgress(r.Context(), currentProfile(r), p); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
