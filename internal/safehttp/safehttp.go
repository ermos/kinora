// Package safehttp builds HTTP transports for URLs that come from third-party sites (scraped pages, hoster
// playlists, sites.json): they refuse to connect to loopback, private, link-local or otherwise non-public
// addresses, so a malicious page cannot make the server fetch its own network (SSRF). The check runs on the
// resolved address of every connection, redirects and DNS rebinding included.
package safehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrBlocked is returned when a connection targets a non-public address.
var ErrBlocked = errors.New("destination address not allowed")

// cgnat is the carrier-grade NAT range (RFC 6598), also used by VPNs such as Tailscale.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// Public reports whether a is a routable internet address.
func Public(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || cgnat.Contains(a) {
		return false
	}
	return !a.Is4() || a.As4()[0] != 0 // 0.0.0.0/8 is "this network"
}

func control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return err
	}
	if !Public(ap.Addr()) {
		return fmt.Errorf("%w: %s", ErrBlocked, ap.Addr())
	}
	return nil
}

// Transport is http.DefaultTransport that only dials public addresses.
func Transport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second, Control: control}).DialContext
	return tr
}
