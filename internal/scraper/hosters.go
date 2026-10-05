package scraper

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// Ports of vStream hosters/*.py. Match lists come from vStream's cHosterGui.checkHoster.
func init() {
	Hosters = append(Hosters,
		Hoster{Name: "Lulustream", Match: []string{"lulustream", "luluvid", "luluvdo", "lulu.st", "streamhihi"}, Resolve: lulustream},
		Hoster{Name: "Vidzy", Match: []string{"vidzy"}, Resolve: vidzy},
		Hoster{Name: "Uqload", Match: []string{"uqload"}, Resolve: uqload},
	)
}

func lulustream(ctx context.Context, c *Client, u string) (Stream, error) {
	html, final, err := c.Get(ctx, u, map[string]string{"Accept": "text/html,application/xhtml+xml"})
	if err != nil {
		return Stream{}, err
	}
	const pattern = `sources:\s*\[\{file:\s*["']([^"']+\.m3u8[^"']*)["']`
	src := Find(html, pattern)
	if src == "" {
		src = Find(Unpack(html), pattern)
	}
	if src == "" {
		return Stream{}, ErrNotFound
	}
	return Stream{URL: src, Headers: map[string]string{
		"Referer": final, "Origin": Origin(final), "Cookie": c.Cookies(final),
	}}, nil
}

func vidzy(ctx context.Context, c *Client, u string) (Stream, error) {
	html, final, err := c.Get(ctx, u, nil)
	if err != nil {
		return Stream{}, err
	}
	src := Find(Unpack(html), `\{src:\s*"([^"]+)"`)
	if src == "" {
		if blob := Find(html, `sources:\s*\[\{src:\s*\(function\(s\).+?\}\)\("([^"]+)"\)`); blob != "" {
			src = vidzyDecode(blob, Host(final))
		}
	}
	if src == "" {
		return Stream{}, ErrNotFound
	}
	return Stream{URL: src, Headers: map[string]string{"Referer": final}}, nil
}

// vidzyDecode reverses the player's obfuscation: base64, reversed, xored with a key seeded from the host
// plus a browser-dependent offset (today the pixel width of a 1in div, 96). The offset is brute-forced so a
// new trick of the same kind keeps working.
func vidzyDecode(blob, host string) string {
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return ""
	}
	seed := 0
	for _, ch := range host {
		seed += int(ch)
	}
	for offset := 0; offset < 256; offset++ {
		out := make([]byte, len(raw))
		for i := range raw {
			out[i] = raw[len(raw)-1-i] ^ byte((0x3d+i*89+seed+offset)&255)
		}
		if s := string(out); strings.HasPrefix(s, "http") && !strings.Contains(s, "/troll/") {
			return s
		}
	}
	return ""
}

func uqload(ctx context.Context, c *Client, u string) (Stream, error) {
	html, final, err := c.Get(ctx, u, nil)
	if err != nil {
		return Stream{}, err
	}
	src := Find(html, `sources.+?"([^"]+mp4)"`)
	if src == "" {
		for _, m := range FindAll(Unpack(html), `file:"([^"]+)"`) {
			if strings.Contains(m[0], "mp4") || strings.Contains(m[0], "m3u8") {
				src = m[0]
				break
			}
		}
	}
	if src == "" {
		return Stream{}, fmt.Errorf("uqload: %w", ErrNotFound)
	}
	return Stream{URL: src, Headers: map[string]string{"Referer": Origin(final) + "/"}}, nil
}
