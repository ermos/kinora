package scraper

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	rePacked   = regexp.MustCompile(`(?s)eval\(function\(p,a,c,k,e,[dr]\).+?\}\('(.*?)',\s*(\d+),\s*(\d+),\s*'(.*?)'\.split\('\|'\)`)
	rePackWord = regexp.MustCompile(`\b\w+\b`)
)

const base62 = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Unpack decodes the first Dean Edwards p.a.c.k.e.r. script found in html (port of vStream's cPacker).
func Unpack(html string) string {
	m := rePacked.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	payload := strings.ReplaceAll(m[1], `\'`, `'`)
	radix, _ := strconv.Atoi(m[2])
	symtab := strings.Split(m[4], "|")
	return rePackWord.ReplaceAllStringFunc(payload, func(w string) string {
		i := unbase(w, radix)
		if i >= 0 && i < len(symtab) && symtab[i] != "" {
			return symtab[i]
		}
		return w
	})
}

func unbase(s string, radix int) int {
	if radix <= 36 {
		n, err := strconv.ParseInt(s, radix, 64)
		if err != nil {
			return -1
		}
		return int(n)
	}
	n := 0
	for _, c := range s {
		d := strings.IndexRune(base62, c)
		if d < 0 || d >= radix {
			return -1
		}
		n = n*radix + d
	}
	return n
}
