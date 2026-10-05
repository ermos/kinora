package fr

import (
	"reflect"
	"testing"
)

func TestEmbedList(t *testing.T) {
	for arr, want := range map[string][]string{
		"'https://a/1', '', 'https://a/3',":      {"https://a/1", "", "https://a/3"},
		"    'https://a/1,    'https://a/2,    ": {"https://a/1", "https://a/2"}, // closing quotes lost
		` "https://a/1", "https://a/2" `:         {"https://a/1", "https://a/2"},
	} {
		if got := embedList(arr); !reflect.DeepEqual(got, want) {
			t.Errorf("embedList(%q) = %q, want %q", arr, got, want)
		}
	}
}
