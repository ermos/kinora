package api

import "net/http"

// errCode is a stable, machine readable error the UI translates (ui/src/i18n). The English message is a
// fallback for other clients. Adding a code means adding it to the enums tag of apiError.
type errCode string

const (
	errInvalidJSON         errCode = "invalid_json"
	errUnsupportedMedia    errCode = "unsupported_media_type"
	errNotLoggedIn         errCode = "not_logged_in"
	errSessionExpired      errCode = "session_expired"
	errAdminOnly           errCode = "admin_only"
	errMissingProfile      errCode = "missing_profile"
	errUnknownProfile      errCode = "unknown_profile"
	errInternal            errCode = "internal"
	errNotFound            errCode = "not_found"
	errInvalidCredentials  errCode = "invalid_credentials" //nolint:gosec // G101: an error code, not a credential
	errAlreadySetUp        errCode = "already_set_up"
	errUsernameRequired    errCode = "username_required"
	errPasswordLength      errCode = "password_length"
	errUnsupportedLanguage errCode = "unsupported_language"
	errWrongPassword       errCode = "wrong_password"
	errNameRequired        errCode = "name_required"
	errUnknownAvatar       errCode = "unknown_avatar"
	errProfileLimit        errCode = "profile_limit"
	errLastProfile         errCode = "last_profile"
	errUsernameTaken       errCode = "username_taken"
	errDeleteSelf          errCode = "delete_self"
	errInvalidID           errCode = "invalid_id"
	errInvalidRequest      errCode = "invalid_request"
	errEpisodeRequired     errCode = "episode_required"
	errTMDBUnreachable     errCode = "tmdb_unreachable"
	errInvalidLink         errCode = "invalid_link"
	errLinkDead            errCode = "link_dead"
	errInvalidProgress     errCode = "invalid_progress"
	errFlareSolverr        errCode = "flaresolverr_unreachable"
	errDeviceCodeExpired   errCode = "device_code_expired"
	errDeviceCodeInvalid   errCode = "device_code_invalid"
	errReleaseUnreachable  errCode = "release_unreachable"
	errTooManyAttempts     errCode = "too_many_attempts"
)

var errMessages = map[errCode]string{
	errInvalidJSON:         "invalid JSON body",
	errUnsupportedMedia:    "content type must be application/json",
	errNotLoggedIn:         "not logged in",
	errSessionExpired:      "session expired",
	errAdminOnly:           "admin only",
	errMissingProfile:      "missing " + profileHeader + " header",
	errUnknownProfile:      "unknown profile",
	errInternal:            "internal error",
	errNotFound:            "not found",
	errInvalidCredentials:  "invalid username or password",
	errAlreadySetUp:        "already set up",
	errUsernameRequired:    "username is required (64 chars max)",
	errPasswordLength:      "password must be 8 to 72 characters",
	errUnsupportedLanguage: "unsupported language",
	errWrongPassword:       "wrong current password",
	errNameRequired:        "name is required (32 chars max)",
	errUnknownAvatar:       "unknown avatar",
	errProfileLimit:        "profile limit reached",
	errLastProfile:         "an account needs at least one profile",
	errUsernameTaken:       "username taken",
	errDeleteSelf:          "you cannot delete your own account",
	errInvalidID:           "invalid id",
	errInvalidRequest:      "invalid request parameters",
	errEpisodeRequired:     "season and episode are required for shows",
	errTMDBUnreachable:     "TMDB is unreachable, check TMDB_API_KEY",
	errInvalidLink:         "invalid link",
	errLinkDead:            "this link does not work anymore, try another one",
	errInvalidProgress:     "invalid progress",
	errFlareSolverr:        "no FlareSolverr server answers at this URL",
	errDeviceCodeExpired:   "this code expired, ask for a new one",
	errDeviceCodeInvalid:   "unknown or expired code",
	errReleaseUnreachable:  "the release source is unreachable, try again later",
	errTooManyAttempts:     "too many failed attempts, try again later",
}

// apiError is the body of every error response.
type apiError struct {
	Code  string `json:"code" enums:"invalid_json,unsupported_media_type,not_logged_in,session_expired,admin_only,missing_profile,unknown_profile,internal,not_found,invalid_credentials,already_set_up,username_required,password_length,unsupported_language,wrong_password,name_required,unknown_avatar,profile_limit,last_profile,username_taken,delete_self,invalid_id,invalid_request,episode_required,tmdb_unreachable,invalid_link,link_dead,invalid_progress,flaresolverr_unreachable,device_code_expired,device_code_invalid,release_unreachable,too_many_attempts"`
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code errCode) {
	writeJSON(w, status, apiError{Code: string(code), Error: errMessages[code]})
}
