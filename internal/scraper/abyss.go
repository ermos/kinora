package scraper

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Abyss (abyss.to, white-labelled on many domains) serves an MP4 that its player rebuilds in a service worker.
// Resolve returns an abyss: URL carrying an AbyssFile, which the stream proxy serves back as a plain MP4.
func init() {
	Hosters = append(Hosters, Hoster{Name: "Abyss", Match: []string{"abyss", "dessins-anime-streaming"}, Resolve: abyss})
}

// AbyssFile tells where each byte of an Abyss video lives. Two layouts exist:
//   - parts: URL is the first part, part n is at URL+n, and the first 64 KiB are encrypted with the file name;
//   - sora (Sora set): URL is "https://<host>/mp4/<md5_id>/<res_id>/<size>/<chunk>", chunk n is fetched from
//     /sora/<size>/<token of URL path + "/n">, in clear.
type AbyssFile struct {
	Size int64  `json:"s"`
	URL  string `json:"u"`
	Part int64  `json:"p"` // part or chunk size
	Sora bool   `json:"o,omitempty"`
}

const abyssEncrypted = 64 << 10

// Piece returns where the byte at off lives: the URL, its offset there, and the last file offset that URL holds.
func (f AbyssFile) Piece(off int64) (u string, from, last int64, err error) {
	n := off / f.Part
	from, last = off-n*f.Part, min(f.Size, (n+1)*f.Part)-1
	if !f.Sora {
		if n > 0 {
			return f.URL + strconv.FormatInt(n, 10), from, last, nil
		}
		return f.URL, from, last, nil
	}
	pu, err := url.Parse(f.URL)
	if err != nil {
		return "", 0, 0, err
	}
	tok := []byte(pu.Path + "/" + strconv.FormatInt(n, 10))
	abyssCipher(digitSeed(f.Size), 0).XORKeyStream(tok, tok)
	tok = []byte(base64.RawStdEncoding.EncodeToString(tok))
	return fmt.Sprintf("%s://%s/sora/%d/%s", pu.Scheme, pu.Host, f.Size, base64.RawStdEncoding.EncodeToString(tok)), from, last, nil
}

// Head returns the cipher of the encrypted head positioned at off, or nil when the bytes from off are in clear.
func (f AbyssFile) Head(off int64) (cipher.Stream, int64) {
	if f.Sora || off >= abyssEncrypted {
		return nil, 0
	}
	return abyssCipher(path.Base(f.URL), off), abyssEncrypted - off
}

// abyssCipher is the player's AES-256-CTR: the key is the hex md5 of seed taken as text, the counter starts
// at its first 16 bytes. offset positions the keystream.
func abyssCipher(seed string, offset int64) cipher.Stream {
	sum := md5.Sum([]byte(seed))
	key := []byte(hex.EncodeToString(sum[:]))
	block, _ := aes.NewCipher(key) // 32 bytes: never fails
	ctr := new(big.Int).SetBytes(key[:16])
	ctr.Add(ctr, big.NewInt(offset/16))
	iv := make([]byte, 16)
	ctr.FillBytes(iv) // hex text keeps the top bit clear: no overflow
	s := cipher.NewCTR(block, iv)
	skip := make([]byte, offset%16)
	s.XORKeyStream(skip, skip)
	return s
}

// digitSeed reproduces a quirk of the player's md5 on numbers: it hashes each digit's value, not its character.
func digitSeed(n int64) string {
	b := []byte(strconv.FormatInt(n, 10))
	for i := range b {
		b[i] -= '0'
	}
	return string(b)
}

type abyssSource struct {
	ResID    int    `json:"res_id"`
	Size     int64  `json:"size"`
	Codec    string `json:"codec"`
	Status   bool   `json:"status"`
	Path     string `json:"path"`
	URL      string `json:"url"`
	PartSize int64  `json:"partSize"`
	Sub      string `json:"sub"`
}

func abyss(ctx context.Context, c *Client, u string) (Stream, error) {
	html, final, err := c.Get(ctx, u, nil)
	if err != nil {
		return Stream{}, err
	}
	raw, err := base64.StdEncoding.DecodeString(Find(html, `const datas = "([^"]+)"`))
	if err != nil || len(raw) == 0 {
		return Stream{}, fmt.Errorf("abyss: %w", ErrNotFound)
	}
	// The player reads it with JSON.parse(atob(datas)): a byte string, not UTF-8.
	var datas struct {
		Slug   string `json:"slug"`
		MD5ID  int64  `json:"md5_id"`
		UserID int64  `json:"user_id"`
		Media  string `json:"media"`
	}
	if err := json.Unmarshal([]byte(latin1(raw)), &datas); err != nil {
		return Stream{}, fmt.Errorf("abyss: %w", err)
	}
	media := latin1Bytes(datas.Media)
	abyssCipher(fmt.Sprintf("%d:%s:%d", datas.UserID, datas.Slug, datas.MD5ID), 0).XORKeyStream(media, media)
	var m struct {
		MP4 struct {
			Sources []abyssSource `json:"sources"`
			Domains []string      `json:"domains"`
		} `json:"mp4"`
	}
	if err := json.Unmarshal(media, &m); err != nil {
		return Stream{}, fmt.Errorf("abyss: media: %w", err)
	}
	rank := func(s abyssSource) int { // h264 first: AV1 does not play everywhere
		if s.Codec == "h264" {
			return 100 + s.ResID
		}
		return s.ResID
	}
	var best *abyssSource
	for i, s := range m.MP4.Sources {
		if s.Status && s.Size > 0 && (best == nil || rank(s) > rank(*best)) {
			best = &m.MP4.Sources[i]
		}
	}
	if best == nil {
		return Stream{}, fmt.Errorf("abyss: %w", ErrNotFound)
	}
	f := AbyssFile{Size: best.Size, URL: strings.TrimRight(best.URL, "/") + "/" + best.Path, Part: best.PartSize}
	if best.Path == "" { // sora layout
		host := ""
		for _, d := range m.MP4.Domains {
			if best.Sub != "" && strings.Contains(d, best.Sub) {
				host = d
			}
		}
		if host == "" {
			if len(m.MP4.Domains) == 0 {
				return Stream{}, fmt.Errorf("abyss: %w", ErrNotFound)
			}
			host = m.MP4.Domains[0] // ponytail: never seen without a matching domain
		}
		const chunk = 2 << 20 // what the player asks for
		f = AbyssFile{Size: best.Size, URL: fmt.Sprintf("https://%s/mp4/%d/%d/%d/%d", host, datas.MD5ID, best.ResID, best.Size, chunk), Part: chunk, Sora: true}
	}
	if f.Part <= 0 {
		return Stream{}, fmt.Errorf("abyss: %w", ErrNotFound)
	}
	b, _ := json.Marshal(f)
	return Stream{URL: "abyss:" + base64.RawURLEncoding.EncodeToString(b), Headers: map[string]string{"Referer": Origin(final) + "/"}}, nil
}

// ParseAbyss reads the AbyssFile of an abyss: stream URL.
func ParseAbyss(u string) (AbyssFile, bool) {
	var f AbyssFile
	rest, ok := strings.CutPrefix(u, "abyss:")
	if !ok {
		return f, false
	}
	b, err := base64.RawURLEncoding.DecodeString(rest)
	if err != nil || json.Unmarshal(b, &f) != nil || f.Size <= 0 || f.Part <= 0 {
		return f, false
	}
	return f, true
}

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

func latin1Bytes(s string) []byte {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		b = append(b, byte(r))
	}
	return b
}
