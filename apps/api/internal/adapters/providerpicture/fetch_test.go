package providerpicture

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

type fixedResolver struct{ addresses []netip.Addr }

func (r fixedResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addresses, nil
}

func TestRejectsUnsafeSourcesBeforeDial(t *testing.T) {
	for _, source := range []string{"http://example.com/a", "https://user:pass@example.com/a", "https://example.com:444/a", "https://127.0.0.1/a", "https://[::1]/a", "https://169.254.169.254/latest", "https://example.com/a#fragment"} {
		t.Run(source, func(t *testing.T) {
			f := New()
			dialed := false
			f.dial = func(context.Context, string, string) (net.Conn, error) {
				dialed = true
				return nil, errors.New("unexpected dial")
			}
			defer func() {
				if dialed {
					t.Error("unsafe source reached socket dial")
				}
			}()
			f.resolver = fixedResolver{[]netip.Addr{netip.MustParseAddr("127.0.0.1")}}
			if _, err := f.Fetch(context.Background(), source); err == nil {
				t.Fatal("unsafe source accepted")
			}
		})
	}
}

func TestFetchPinsValidatedAddressAndRejectsRedirect(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		requested := 0
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requested++
			if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Error("credentials forwarded")
			}
			if redirect {
				w.Header().Set("Location", "https://127.0.0.1/private")
				w.WriteHeader(302)
				return
			}
			_, _ = w.Write([]byte("image bytes"))
		}))
		f := New()
		f.resolver = fixedResolver{[]netip.Addr{netip.MustParseAddr("93.184.216.34")}}
		roots := x509.NewCertPool()
		roots.AddCert(server.Certificate())
		f.roots = roots
		f.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != "93.184.216.34:443" {
				t.Errorf("unvalidated dial: %s", address)
			}
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}
		data, err := f.Fetch(context.Background(), "https://example.com/photo")
		server.Close()
		if redirect {
			if err == nil {
				t.Fatal("redirect accepted")
			}
		} else if err != nil || string(data) != "image bytes" {
			t.Fatalf("fetch failed: %v", err)
		}
		if requested != 1 {
			t.Fatalf("requests=%d", requested)
		}
	}
}

func TestRejectsMixedPrivateDNSAndOverBudgetResponse(t *testing.T) {
	f := New()
	f.resolver = fixedResolver{[]netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("10.0.0.1")}}
	f.dial = func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("mixed private DNS reached dial")
		return nil, nil
	}
	if _, err := f.Fetch(context.Background(), "https://example.com/photo"); err == nil {
		t.Fatal("mixed private answer accepted")
	}
	for _, declared := range []bool{false, true} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if declared {
				w.Header().Set("Content-Length", "10000001")
				w.WriteHeader(200)
				return
			}
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			_, _ = io.Copy(w, bytes.NewReader(make([]byte, profilepicture.MaxUploadBytes+1)))
		}))
		f := New()
		f.resolver = fixedResolver{[]netip.Addr{netip.MustParseAddr("93.184.216.34")}}
		roots := x509.NewCertPool()
		roots.AddCert(server.Certificate())
		f.roots = roots
		f.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}
		if _, err := f.Fetch(context.Background(), "https://example.com/photo"); err == nil {
			t.Fatal("oversized response accepted")
		}
		server.Close()
	}
}
