package api

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ermos/kinora/internal/store"
)

const (
	// deviceCodeTTL: the TV asks for a new code once it expires, so the code shown keeps rotating.
	deviceCodeTTL = 5 * time.Minute
	// deviceCodeAlphabet leaves out 0/O and 1/I, easy to mix up when read off a TV.
	deviceCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

type deviceCode struct {
	// Code to enter on the other device, "ABCD-EFGH".
	Code string `json:"code"`
	// Token is the device's secret for polling, never shown.
	Token     string `json:"token"`
	ExpiresIn int    `json:"expiresIn"` // seconds
}

type devicePoll struct {
	Token string `json:"token"`
}

type devicePollResult struct {
	// Approved: the session cookie is set and User is the account.
	Approved bool        `json:"approved"`
	User     *store.User `json:"user,omitempty" validate:"optional"`
}

type deviceApproval struct {
	Code string `json:"code"`
}

// @Summary      Start signing in from another device
// @Description  The TV shows the code and polls /auth/device/poll with the token until a signed-in user approves
// @Description  the code on /link, or it expires.
// @Tags         auth
// @Success      200  {object}  deviceCode
// @Router       /auth/device [post]
func (h *Handler) deviceStart(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		internalError(w, err)
		return
	}
	code := make([]byte, len(b))
	for i, v := range b {
		code[i] = deviceCodeAlphabet[int(v)%len(deviceCodeAlphabet)] // 256 is a multiple of 32: no bias
	}
	token, err := randomToken()
	if err != nil {
		internalError(w, err)
		return
	}
	if err := h.store.CreateDeviceCode(r.Context(), string(code), hashToken(token), time.Now().Add(deviceCodeTTL)); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deviceCode{Code: string(code[:4]) + "-" + string(code[4:]), Token: token, ExpiresIn: int(deviceCodeTTL.Seconds())})
}

// @Summary      Poll a device sign-in
// @Description  Not approved yet: approved is false. Approved: the session cookie is set, like /auth/login.
// @Tags         auth
// @Param        body  body      devicePoll  true  "Token from /auth/device"
// @Success      200   {object}  devicePollResult
// @Failure      410   {object}  apiError  "Expired: start over"
// @Router       /auth/device/poll [post]
func (h *Handler) devicePoll(w http.ResponseWriter, r *http.Request) {
	var p devicePoll
	if !readJSON(w, r, &p) {
		return
	}
	userID, err := h.store.ClaimDeviceCode(r.Context(), hashToken(p.Token))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusGone, errDeviceCodeExpired)
		return
	case err != nil:
		internalError(w, err)
		return
	case userID == 0:
		writeJSON(w, http.StatusOK, devicePollResult{})
		return
	}
	u, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		internalError(w, err)
		return
	}
	if err := h.startSession(w, r, userID); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, devicePollResult{Approved: true, User: &u})
}

// @Summary  Sign a device in with the code it shows
// @Tags     auth
// @Param    body  body  deviceApproval  true  "Code shown on the device"
// @Success  204
// @Failure  404  {object}  apiError  "Unknown or expired code"
// @Router   /auth/device/approve [post]
func (h *Handler) deviceApprove(w http.ResponseWriter, r *http.Request) {
	var a deviceApproval
	if !readJSON(w, r, &a) {
		return
	}
	code := strings.Map(func(c rune) rune {
		if c == '-' || c == ' ' {
			return -1
		}
		return c
	}, strings.ToUpper(a.Code))
	err := h.store.ApproveDeviceCode(r.Context(), code, currentUser(r).ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, errDeviceCodeInvalid)
	case err != nil:
		internalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
