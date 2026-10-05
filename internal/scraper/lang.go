package scraper

// Language is an instance language: the language of the catalog (TMDB), of the UI, and of the sources
// that get queried. A language is offered at setup only once it is supported end to end: at least one
// source, UI strings (ui/src/i18n) and home row titles (api/i18n.go).
type Language struct {
	Code  string // ISO 639-1, also the UI dictionary key
	Label string // native name, shown in the setup language picker
	TMDB  string // TMDB locale for titles, overviews and genres
	// AudioRank orders the language tags sources put on links ("VF", "VOSTFR"...): higher plays first.
	AudioRank func(tag string) int
}

// Languages is filled by the language packages (fr, en) when imported.
var Languages []Language

// DefaultLanguage is used by instances set up before languages existed.
const DefaultLanguage = "fr"

func LanguageByCode(code string) (Language, bool) {
	for _, l := range Languages {
		if l.Code == code {
			return l, true
		}
	}
	return Language{}, false
}

// SupportedLanguages lists the languages at least one source serves.
func SupportedLanguages() []Language {
	var out []Language
	for _, l := range Languages {
		for _, s := range Sources {
			if s.serves(l.Code) {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

func (s Source) serves(lang string) bool {
	for _, l := range s.Langs {
		if l == lang {
			return true
		}
	}
	return false
}
