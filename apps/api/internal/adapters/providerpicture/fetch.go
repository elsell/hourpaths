package providerpicture

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// Fetcher retrieves untrusted provider media without exposing internal services.
// Resolution is validated once and dialing uses only the resulting IP literals.
type Fetcher struct {
	resolver resolver
	dial     func(context.Context, string, string) (net.Conn, error)
	roots    *x509.CertPool
}

func New() *Fetcher {
	return &Fetcher{resolver: net.DefaultResolver, dial: (&net.Dialer{Timeout: 3 * time.Second}).DialContext}
}

var excluded = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func public(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	if address.Is6() && !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}
	for _, block := range excluded {
		if block.Contains(address) {
			return false
		}
	}
	return true
}
func (f *Fetcher) Fetch(ctx context.Context, source string) ([]byte, error) {
	u, err := url.Parse(source)
	if err != nil || len(source) > 4096 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || strings.Contains(u.Hostname(), "%") {
		return nil, ports.ErrInvalidArgument
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var addresses []netip.Addr
	if literal, e := netip.ParseAddr(u.Hostname()); e == nil {
		addresses = []netip.Addr{literal}
	} else {
		addresses, err = f.resolver.LookupNetIP(ctx, "ip", u.Hostname())
	}
	if err != nil || len(addresses) == 0 {
		return nil, ports.ErrUnavailable
	}
	for _, address := range addresses {
		if !public(address) {
			return nil, ports.ErrInvalidArgument
		}
	}
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.roots},
		TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 3 * time.Second, DisableCompression: true,
		MaxResponseHeaderBytes: 16384,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			// No second DNS lookup and no environment-controlled HTTP proxy.
			for _, ip := range addresses {
				connection, err := f.dial(ctx, network, net.JoinHostPort(ip.String(), "443"))
				if err == nil {
					return connection, nil
				}
			}
			return nil, ports.ErrUnavailable
		},
	}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ports.ErrInvalidArgument
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, ports.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > profilepicture.MaxUploadBytes {
		return nil, ports.ErrInvalidArgument
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, profilepicture.MaxUploadBytes+1))
	if err != nil || len(data) == 0 || len(data) > profilepicture.MaxUploadBytes {
		return nil, ports.ErrInvalidArgument
	}
	return data, nil
}
