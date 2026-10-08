package scraper

import (
	"net/http"
	"os"
	"testing"
)

// The tests serve their pages from httptest servers on loopback, which the production transport refuses.
func TestMain(m *testing.M) {
	transport = http.DefaultTransport
	os.Exit(m.Run())
}
