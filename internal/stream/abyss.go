package stream

import (
	"crypto/cipher"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ermos/kinora/internal/scraper"
)

// serveAbyss serves an Abyss video (see scraper.AbyssFile) as one plain MP4, fetching each piece upstream
// and decrypting the encrypted head on the fly.
func (p *Proxy) serveAbyss(w http.ResponseWriter, r *http.Request, f scraper.AbyssFile, headers map[string]string) {
	start, end, ok := parseRange(r.Header.Get("Range"), f.Size)
	if !ok {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", f.Size))
		http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	status := http.StatusOK
	if r.Header.Get("Range") != "" {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, f.Size))
		status = http.StatusPartialContent
	}
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}

	for off := start; off <= end; {
		u, from, last, err := f.Piece(off)
		if err != nil {
			return
		}
		last = min(last, end)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", scraper.UserAgent)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, from+last-off))
		resp, err := p.client.Do(req)
		if err != nil {
			return // headers are sent: the player sees a short body and asks again from there
		}
		if resp.StatusCode != http.StatusPartialContent { // an error page or the whole part: not the bytes asked for
			resp.Body.Close()
			return
		}
		body := io.LimitReader(resp.Body, last-off+1)
		if s, n := f.Head(off); s != nil {
			body = &decrypter{r: body, s: s, left: n}
		}
		n, _ := io.Copy(w, body)
		resp.Body.Close()
		if n != last-off+1 {
			return
		}
		off = last + 1
	}
}

// decrypter decrypts the first left bytes read from r and passes the rest through.
type decrypter struct {
	r    io.Reader
	s    cipher.Stream
	left int64
}

func (d *decrypter) Read(b []byte) (int, error) {
	n, err := d.r.Read(b)
	if k := min(int64(n), d.left); k > 0 {
		d.s.XORKeyStream(b[:k], b[:k])
		d.left -= k
	}
	return n, err
}

// parseRange reads a single "bytes=a-b", "bytes=a-" or "bytes=-n" range; no header means the whole file.
func parseRange(h string, size int64) (start, end int64, ok bool) {
	if h == "" {
		return 0, size - 1, true
	}
	spec, found := strings.CutPrefix(h, "bytes=")
	a, b, dash := strings.Cut(spec, "-")
	if !found || !dash || strings.Contains(spec, ",") {
		return 0, 0, false
	}
	if a == "" { // suffix: the last n bytes
		n, err := strconv.ParseInt(b, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		return max(0, size-n), size - 1, true
	}
	start, err := strconv.ParseInt(a, 10, 64)
	if err != nil || start >= size {
		return 0, 0, false
	}
	end = size - 1
	if b != "" {
		if end, err = strconv.ParseInt(b, 10, 64); err != nil || end < start {
			return 0, 0, false
		}
		end = min(end, size-1)
	}
	return start, end, true
}
