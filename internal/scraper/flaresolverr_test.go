package scraper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A challenged request goes through FlareSolverr once; the next ones reuse its clearance directly.
func TestCloudflareChallengeSolvedOnce(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("cf_clearance"); err != nil || c.Value != "ok" || r.UserAgent() != "browser" {
			w.Header().Set("Cf-Mitigated", "challenge")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte("direct"))
	}))
	defer site.Close()

	solves := 0
	solver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		solves++
		var cmd struct{ URL string }
		json.NewDecoder(r.Body).Decode(&cmd)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "solution": map[string]any{
			"url": cmd.URL, "status": 200, "response": "solved", "userAgent": "browser",
			"cookies": []map[string]string{{"name": "cf_clearance", "value": "ok"}},
		}})
	}))
	defer solver.Close()

	get := func(withSolver bool) (string, error) {
		c := NewClient()
		if withSolver {
			c.Solver = solver.URL
		}
		page, _, err := c.Get(context.Background(), site.URL+"/page", nil)
		return page, err
	}
	if _, err := get(false); err == nil {
		t.Fatal("challenge without solver should fail")
	}
	for i, want := range []string{"solved", "direct"} {
		if page, err := get(true); err != nil || page != want {
			t.Fatalf("request %d = %q, %v; want %q", i, page, err, want)
		}
	}
	if solves != 1 {
		t.Fatalf("solver called %d times, want 1", solves)
	}
}
