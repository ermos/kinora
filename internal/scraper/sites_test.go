package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Each source syncs from its own list; a list that fails leaves the others working.
func TestFetchSiteURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fr.json":
			w.Write([]byte(`{"sites":{"a":{"url":"https://a.fr"},"b":{"url":"https://wrong.fr"}}}`))
		case "/en.json":
			w.Write([]byte(`{"sites":{"b":{"url":"https://b.com/"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	saved := Sources
	defer func() { Sources = saved }()
	Sources = []Source{{ID: "a", Sites: srv.URL + "/fr.json"}, {ID: "b", Sites: srv.URL + "/en.json"}, {ID: "c", Sites: srv.URL + "/gone.json"}}

	got, err := FetchSiteURLs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["a"] != "https://a.fr/" || got["b"] != "https://b.com/" {
		t.Fatalf("got %v", got)
	}
}
