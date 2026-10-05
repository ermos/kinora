package scraper

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestUnpack(t *testing.T) {
	packed := `<script>eval(function(p,a,c,k,e,d){while(c--)if(k[c])p=p.replace(new RegExp('\\b'+c.toString(a)+'\\b','g'),k[c]);return p}('0 1={2:"3://4.5/6.7"};',8,8,'var|player|file|https|cdn|net|master|m3u8'.split('|'),0,{}))</script>`
	got := Unpack(packed)
	want := `var player={file:"https://cdn.net/master.m3u8"};`
	if got != want {
		t.Fatalf("Unpack = %q, want %q", got, want)
	}
}

func TestVidzyDecode(t *testing.T) {
	// Encode like the player does, with the browser offset (96) the decoder has to find.
	const host, want = "vidzy.org", "https://u1.vidzy.cc/hls2/master.m3u8"
	seed := 0
	for _, ch := range host {
		seed += int(ch)
	}
	enc := make([]byte, len(want))
	for i := range want {
		enc[len(want)-1-i] = want[i] ^ byte((0x3d+i*89+seed+96)&255)
	}
	if got := vidzyDecode(base64.StdEncoding.EncodeToString(enc), host); got != want {
		t.Fatalf("vidzyDecode = %q, want %q", got, want)
	}
}

func TestQueryMatching(t *testing.T) {
	q := Query{Title: "Le Seigneur des anneaux : La Communauté de l'anneau", OriginalTitle: "The Lord of the Rings: The Fellowship of the Ring", Year: 2001}
	if !q.SameTitle("Le Seigneur des Anneaux - La communaute de l'Anneau") || !q.SameTitle("The Lord of the Rings The Fellowship of the Ring") {
		t.Fatal("expected titles to match")
	}
	if q.SameTitle("Le Seigneur des anneaux : Les Deux Tours") {
		t.Fatal("different title matched")
	}
	if !q.SameYear(2002) || q.SameYear(2003) || !q.SameYear(0) {
		t.Fatal("year tolerance is one year, unknown years match")
	}
}

func TestFindAllCleansLikeVStream(t *testing.T) {
	html := "<a\n href=\"https:\\/\\/x.com\\/a?b=1&amp;c=2\">\n\tTitre</a>"
	got := FindAll(html, `href="([^"]+)">(.+?)</A>`)
	if len(got) != 1 || got[0][0] != "https://x.com/a?b=1&c=2" || got[0][1] != "Titre" {
		t.Fatalf("FindAll = %q", got)
	}
}

func TestHosterFor(t *testing.T) {
	for url, want := range map[string]string{
		"https://luluvdo.com/e/abc":             "Lulustream",
		"https://Uqload.cx/embed-x.html":        "Uqload",
		"https://cdn.example/movie/master.m3u8": "Direct",
	} {
		if h := HosterFor(url); h == nil || h.Name != want {
			t.Errorf("HosterFor(%s) = %v, want %s", url, h, want)
		}
	}
	if h := HosterFor("https://unknown.example/e/x"); h == nil || h.Name != "Lecteur" {
		t.Error("unknown embed URLs are sniffed")
	}
	if HosterFor("https://unknown.example/watch?v=1") != nil {
		t.Error("non-embed URLs should not match")
	}
}

func TestQualityTiers(t *testing.T) {
	master := "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1,RESOLUTION=1280x534\nlow.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=2,RESOLUTION=1920x800\nhi.m3u8\n"
	if got := hlsTier(master); got != Quality1080 {
		t.Errorf("scope 1920x800 should be 1080p, got %d", got)
	}
	for label, want := range map[string]int{"pulse | 1080p | MULTI": Quality1080, "4K HDR": Quality4K, "HD": Quality720, "CAM": QualitySD, "VF": QualityUnknown} {
		if got := tierFromLabel(label); got != want {
			t.Errorf("tierFromLabel(%q) = %d, want %d", label, got, want)
		}
	}
}

func TestSortLinks(t *testing.T) {
	ls := []Link{
		{URL: "dead", Lang: "VF", Tier: Quality4K, Dead: true},
		{URL: "vostfr-4k", Lang: "VOSTFR", Tier: Quality4K},
		{URL: "vf-720", Lang: "VF", Tier: Quality720},
		{URL: "multi-1080", Lang: "MULTI", Tier: Quality1080},
		{URL: "vf-unknown", Lang: "VF"},
		{URL: "nolang-1080", Tier: Quality1080},
	}
	SortLinks(ls, frenchAudio)
	var got []string
	for _, l := range ls {
		got = append(got, l.URL)
	}
	want := []string{"multi-1080", "nolang-1080", "vf-720", "vf-unknown", "vostfr-4k", "dead"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("SortLinks = %v, want %v", got, want)
	}
}
