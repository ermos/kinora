package tmdb

import (
	"encoding/json"
	"testing"
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
