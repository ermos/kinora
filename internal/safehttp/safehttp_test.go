package safehttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestPublic(t *testing.T) {
	for addr, want := range map[string]bool{
		"1.1.1.1": true, "2606:4700:4700::1111": true, "100.63.255.255": true,
		"127.0.0.1": false, "::1": false, "10.0.0.1": false, "172.17.0.1": false, "192.168.1.1": false,
		"169.254.169.254": false, "100.64.0.1": false, "0.0.0.0": false, "0.1.2.3": false, "224.0.0.1": false,
		"fc00::1": false, "fe80::1": false, "::ffff:127.0.0.1": false, "::ffff:8.8.8.8": true,
	} {
		if got := Public(netip.MustParseAddr(addr)); got != want {
			t.Errorf("Public(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestTransportRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	_, err := (&http.Client{Transport: Transport()}).Get(srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("got %v, want ErrBlocked", err)
	}
}
