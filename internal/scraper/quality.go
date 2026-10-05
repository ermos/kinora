package scraper

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Quality tiers, best first. Width drives the tier: 1920x800 (scope) is still a 1080p encode.
const (
	QualityUnknown = 0
	QualitySD      = 1
	Quality720     = 2
	Quality1080    = 3
	Quality4K      = 4
)

var qualityLabels = map[int]string{QualitySD: "SD", Quality720: "720p", Quality1080: "1080p", Quality4K: "4K"}

func QualityLabel(tier int) string { return qualityLabels[tier] }

func tierFromWidth(w int) int {
	switch {
	case w >= 3200:
		return Quality4K
	case w >= 1700:
		return Quality1080
	case w >= 1200:
		return Quality720
	case w > 0:
		return QualitySD
	}
	return QualityUnknown
}

var reDeclared = regexp.MustCompile(`(?i)\b(2160p?|4k|uhd|1080p?|fhd|720p?|hd|480p?|360p?|sd|cam|ts|hdcam)\b`)

// tierFromLabel reads a quality the site declares ("1080p", "HD", "4K"...).
func tierFromLabel(s string) int {
	best := QualityUnknown
	for _, m := range reDeclared.FindAllString(s, -1) {
		t := QualityUnknown
		switch strings.ToLower(strings.TrimSuffix(strings.ToLower(m), "p")) {
		case "2160", "4k", "uhd":
			t = Quality4K
		case "1080", "fhd":
			t = Quality1080
		case "720", "hd":
			t = Quality720
		case "480", "360", "sd", "cam", "ts", "hdcam":
			t = QualitySD
		}
		best = max(best, t)
	}
	return best
}

var reStreamInf = regexp.MustCompile(`RESOLUTION=(\d+)x(\d+)`)

// hlsTier returns the best variant of an HLS master playlist (unknown for a media playlist).
func hlsTier(playlist string) int {
	best := QualityUnknown
	for _, m := range reStreamInf.FindAllStringSubmatch(playlist, -1) {
		w, _ := strconv.Atoi(m[1])
		best = max(best, tierFromWidth(w))
	}
	return best
}

// ProbeTimeout bounds how long FindLinks spends resolving links to measure their quality.
const ProbeTimeout = 10 * time.Second

// probe resolves every link concurrently: it measures the real quality of HLS streams, flags dead links
// and keeps the resolved stream so playback starts without resolving again. Links not probed in time keep
// the quality their site declared.
func probe(ctx context.Context, links []Link) {
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	sem := make(chan struct{}, 8) // ponytail: fixed concurrency, enough for a handful of sources
	var wg sync.WaitGroup
	for i := range links {
		l := &links[i]
		l.Tier = tierFromLabel(l.Quality)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			st, err := Resolve(ctx, *l)
			if err != nil {
				if ctx.Err() == nil { // a timeout says nothing about the link
					l.Dead = true
				}
				return
			}
			l.Stream, l.ResolvedAt = &st, time.Now().Unix()
			if isHLS(st.URL) {
				body, _, err := NewClient().Get(ctx, st.URL, st.Headers)
				switch {
				case err != nil && ctx.Err() == nil: // the hoster answered but its CDN does not: unplayable
					l.Dead = true
				case err == nil:
					if t := hlsTier(body); t != QualityUnknown {
						l.Tier = t
					}
				}
			}
		}()
	}
	wg.Wait()
	for i := range links {
		if l := &links[i]; l.Tier != QualityUnknown {
			l.Quality = QualityLabel(l.Tier)
		}
	}
}

func isHLS(u string) bool {
	return strings.HasSuffix(strings.ToLower(strings.SplitN(u, "?", 2)[0]), ".m3u8")
}

// SortLinks orders links for playback: by the instance language's audio preference, then best quality first;
// dead links last.
func SortLinks(ls []Link, audioRank func(tag string) int) {
	sort.SliceStable(ls, func(i, j int) bool {
		a, b := ls[i], ls[j]
		if a.Dead != b.Dead {
			return !a.Dead
		}
		if ra, rb := audioRank(a.Lang), audioRank(b.Lang); ra != rb {
			return ra > rb
		}
		return a.Tier > b.Tier
	})
}
