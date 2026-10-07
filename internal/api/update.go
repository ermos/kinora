package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// release is the latest TV app build, for the in-app update prompt.
type release struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
	// APK download URL: absolute, or in development a signed, expiring path on this server (the native downloader
	// does not send the session cookie).
	APK string `json:"apk"`
}

const (
	githubRelease = "https://api.github.com/repos/ermos/kinora/releases/latest"
	releaseTTL    = 24 * time.Hour
	// localBuild is where `npm run tv:apk` leaves the APK, seen from the repo root (make run).
	localBuild = "ui/android/app/build/outputs/apk/release"
	apkTTL     = time.Hour
)

// apkToken is what the signed local APK URL carries.
type apkToken struct {
	Exp int64 `json:"e"`
}

// releases finds the latest release: the local build in development, GitHub's latest release otherwise, kept a day
// since every TV asks on launch and GitHub allows 60 unauthenticated calls an hour (an admin can force a check).
type releases struct {
	dev    bool
	github string
	local  string
	client *http.Client

	mu      sync.Mutex
	latest  *release
	fetched time.Time
}

func newReleases(dev bool) *releases {
	return &releases{dev: dev, github: githubRelease, local: localBuild, client: &http.Client{Timeout: 10 * time.Second}}
}

// get returns the latest release, nil when there is none with an APK. force skips the cache.
func (rs *releases) get(r *http.Request, force bool) (*release, error) {
	if rs.dev {
		return rs.localRelease()
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if !force && !rs.fetched.IsZero() && time.Since(rs.fetched) < releaseTTL {
		return rs.latest, nil
	}
	rel, err := rs.githubRelease(r)
	if err != nil {
		return nil, err
	}
	rs.latest, rs.fetched = rel, time.Now()
	return rel, nil
}

func (rs *releases) githubRelease(r *http.Request) (*release, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, rs.github, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := rs.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github release: %s", res.Status)
	}
	var gh struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(res.Body).Decode(&gh); err != nil {
		return nil, err
	}
	for _, a := range gh.Assets {
		if gh.TagName != "" && strings.HasSuffix(a.Name, ".apk") {
			return &release{Version: strings.TrimPrefix(gh.TagName, "v"), Notes: gh.Body, APK: a.URL}, nil
		}
	}
	return nil, nil
}

// localRelease describes the local build from the metadata Gradle writes next to the APK.
func (rs *releases) localRelease() (*release, error) {
	b, err := os.ReadFile(filepath.Join(rs.local, "output-metadata.json"))
	if os.IsNotExist(err) {
		return nil, nil // nothing built yet
	}
	if err != nil {
		return nil, err
	}
	var meta struct {
		Elements []struct {
			VersionName string `json:"versionName"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	if len(meta.Elements) == 0 || meta.Elements[0].VersionName == "" {
		return nil, nil
	}
	if _, err := os.Stat(rs.localAPK()); err != nil {
		return nil, nil
	}
	return &release{Version: meta.Elements[0].VersionName, Notes: "Local build"}, nil
}

func (rs *releases) localAPK() string { return filepath.Join(rs.local, "app-release.apk") }

// @Summary  Latest TV app release, for the in-app update prompt
// @Tags     instance
// @Success  200  {object}  release
// @Success  204  "No release available, or the release source is unreachable"
// @Failure  401  {object}  apiError
// @Router   /update [get]
func (h *Handler) latestRelease(w http.ResponseWriter, r *http.Request) {
	rel, err := h.releases.get(r, false)
	if err != nil {
		slog.Warn("latest release unavailable", "err", err) // the TV just skips the prompt until next launch
	}
	h.writeRelease(w, rel)
}

// @Summary  Check for a new TV app release now, skipping the cache
// @Tags     admin
// @Success  200  {object}  release
// @Success  204  "No release available"
// @Failure  502  {object}  apiError
// @Router   /admin/update/check [post]
func (h *Handler) checkRelease(w http.ResponseWriter, r *http.Request) {
	rel, err := h.releases.get(r, true)
	if err != nil {
		slog.Warn("release check failed", "err", err)
		writeError(w, http.StatusBadGateway, errReleaseUnreachable)
		return
	}
	h.writeRelease(w, rel)
}

func (h *Handler) writeRelease(w http.ResponseWriter, rel *release) {
	if rel == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if h.releases.dev {
		tok, err := h.signer.Sign(apkToken{Exp: time.Now().Add(apkTTL).Unix()})
		if err != nil {
			internalError(w, err)
			return
		}
		rel.APK = "/api/v1/update/apk?token=" + url.QueryEscape(tok)
	}
	writeJSON(w, http.StatusOK, rel)
}

// @Summary  The locally built TV APK (development only), at the signed URL given by /update
// @Tags     instance
// @Param    token  query  string  true  "Signed token from /update"
// @Produce  application/vnd.android.package-archive
// @Success  200
// @Failure  401  {object}  apiError
// @Router   /update/apk [get]
func (h *Handler) localAPK(w http.ResponseWriter, r *http.Request) {
	var tok apkToken
	if err := h.signer.Verify(r.URL.Query().Get("token"), &tok); err != nil || time.Now().Unix() > tok.Exp {
		writeError(w, http.StatusUnauthorized, errNotLoggedIn)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	http.ServeFile(w, r, h.releases.localAPK())
}
