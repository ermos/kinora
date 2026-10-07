package fr

import (
	"reflect"
	"testing"

	"github.com/ermos/kinora/internal/scraper"
)

func TestToroPlayers(t *testing.T) {
	html := `<ul class="TPlayerNv"><li data-tplayernv="Opt1"><span>Lecteur 2</span><span>VF</span></li>` +
		`<li data-tplayernv="Opt2"><span>Voir 2</span><span>VOSTFR</span></li></ul>` +
		`<div id="Opt1"><iframe src="https://s.com/?trembed=0&amp;trid=9&amp;trtype=2"></iframe></div>` +
		`<div id="Opt2">&lt;iframe src="https://s.com/?trembed=1&amp;#038;trid=9&amp;#038;trtype=2"&gt;</div>`
	want := []scraper.Link{
		{URL: "https://s.com/?trembed=0&trid=9&trtype=2", Lang: "VF"},
		{URL: "https://s.com/?trembed=1&trid=9&trtype=2", Lang: "VOSTFR"},
	}
	if got := toroPlayers(html, "https://s.com/"); !reflect.DeepEqual(got, want) {
		t.Errorf("toroPlayers() = %+v, want %+v", got, want)
	}
}
