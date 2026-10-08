package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ermos/kinora/internal/dbtest"
	"github.com/ermos/kinora/internal/store"
)

// requireProfile is what keeps a profile's history, lists and ratings to its account.
func TestRequireProfile(t *testing.T) {
	st, ctx := store.New(dbtest.Open(t)), context.Background()
	alice, _ := st.CreateUser(ctx, "alice", "h", false)
	bob, _ := st.CreateUser(ctx, "bob", "h", false)
	mine, _ := st.CreateProfile(ctx, alice.ID, "a", "red", true)
	theirs, _ := st.CreateProfile(ctx, bob.ID, "b", "blue", true)

	h := &Handler{store: st}
	var got int64
	protected := h.requireProfile(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = currentProfile(r) }))
	for _, tc := range []struct {
		header string
		want   int
	}{
		{strconv.FormatInt(mine.ID, 10), http.StatusOK},
		{strconv.FormatInt(theirs.ID, 10), http.StatusForbidden},
		{"999999", http.StatusForbidden},
		{"", http.StatusBadRequest},
		{"x", http.StatusBadRequest},
	} {
		got = 0
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), userKey, alice))
		if tc.header != "" {
			r.Header.Set(profileHeader, tc.header)
		}
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, r)
		if rec.Code != tc.want || (tc.want == http.StatusOK) != (got == mine.ID) {
			t.Errorf("profile %q: status %d, profile %d; want %d", tc.header, rec.Code, got, tc.want)
		}
	}
}
