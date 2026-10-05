package tmdb

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLogoPicksCatalogLanguageThenEnglishThenTextless(t *testing.T) {
	var im images
	_ = json.Unmarshal([]byte(`{"logos":[
		{"iso_639_1":"","file_path":"/none.png"},
		{"iso_639_1":"en","file_path":"/en.png"},
		{"iso_639_1":"fr","file_path":"/fr.svg"},
		{"iso_639_1":"fr","file_path":"/fr.png"}]}`), &im)
	for lang, want := range map[string]string{"fr": "/fr.png", "de": "/en.png"} {
		if got := im.logo(lang); got != want {
			t.Errorf("logo(%q) = %q, want %q", lang, got, want)
		}
	}
	im.Logos = im.Logos[:1]
	if got := im.logo("fr"); got != "/none.png" {
		t.Errorf("textless fallback = %q", got)
	}
}

func TestRecent(t *testing.T) {
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	for date, want := range map[string]bool{
		"2026-10-05": true,  // today
		"2026-09-05": true,  // 30 days ago
		"2026-09-04": false, // 31 days ago
		"2026-10-06": false, // not released yet
		"":           false,
	} {
		if got := recent(date, 30, now); got != want {
			t.Errorf("recent(%q) = %v, want %v", date, got, want)
		}
	}
}
