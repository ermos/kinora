package api

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ermos/kinora/internal/scraper"
)

// Adding a language to scraper.Languages must come with every home row title.
func TestLanguagesTranslateEveryRow(t *testing.T) {
	for _, l := range scraper.Languages {
		for _, specs := range homeRows {
			for _, s := range specs {
				if _, ok := rowTitles[l.Code][s.key]; !ok {
					t.Errorf("%s: missing row title %q", l.Code, s.key)
				}
			}
		}
	}
}

// Every error code needs an English message and must be listed in the enums tag, which is what makes the UI
// build fail until the code is translated.
func TestErrorCodesAreDeclared(t *testing.T) {
	field, _ := reflect.TypeOf(apiError{}).FieldByName("Code")
	enums := map[string]bool{}
	for _, c := range strings.Split(field.Tag.Get("enums"), ",") {
		enums[c] = true
	}
	for code, msg := range errMessages {
		if msg == "" || !enums[string(code)] {
			t.Errorf("error code %q: message %q, in enums tag: %v", code, msg, enums[string(code)])
		}
	}
	if len(enums) != len(errMessages) {
		t.Errorf("enums tag lists %d codes, errMessages %d", len(enums), len(errMessages))
	}
}
