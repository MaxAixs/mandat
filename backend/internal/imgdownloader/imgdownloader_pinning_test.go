package imgdownloader

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinResolver is a fake DNS resolver returning fixed addresses and counting
// lookups, so tests prove which address is actually dialed.
type pinResolver struct {
	addrs []netip.Addr
	err   error
	calls int
}

func (r *pinResolver) LookupNetIP(_ context.Context, _, _ string) ([]netip.Addr, error) {
	r.calls++

	return r.addrs, r.err
}

func TestIsPublicAddr_Behavior(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "public ipv4 google", ip: "8.8.8.8", want: true},
		{name: "public ipv4 cloudflare", ip: "1.1.1.1", want: true},
		{name: "public ipv6 cloudflare", ip: "2606:4700::1111", want: true},
		{name: "loopback ipv4", ip: "127.0.0.1", want: false},
		{name: "private 10/8", ip: "10.0.0.1", want: false},
		{name: "private 172.16/12", ip: "172.16.0.1", want: false},
		{name: "private 192.168/16", ip: "192.168.1.1", want: false},
		{name: "link local ipv4", ip: "169.254.169.254", want: false},
		{name: "shared cgnat space", ip: "100.64.0.1", want: false},
		{name: "unspecified ipv4", ip: "0.0.0.0", want: false},
		{name: "ietf protocol assignments", ip: "192.0.0.1", want: false},
		{name: "test net 1", ip: "192.0.2.1", want: false},
		{name: "benchmarking", ip: "198.18.0.1", want: false},
		{name: "test net 2", ip: "198.51.100.1", want: false},
		{name: "test net 3", ip: "203.0.113.1", want: false},
		{name: "class e reserved", ip: "240.0.0.1", want: false},
		{name: "broadcast", ip: "255.255.255.255", want: false},
		{name: "unspecified ipv6", ip: "::", want: false},
		{name: "loopback ipv6", ip: "::1", want: false},
		{name: "link local ipv6", ip: "fe80::1", want: false},
		{name: "unique local ipv6", ip: "fc00::1", want: false},
		{name: "unique local ipv6 documentation", ip: "fd12::1", want: false},
		{name: "well known prefix nat64", ip: "64:ff9b::7f00:1", want: false},
		{name: "local use nat64", ip: "64:ff9b:1::1", want: false},
		{name: "discard prefix", ip: "100::1", want: false},
		{name: "teredo", ip: "2001::1", want: false},
		{name: "documentation ipv6", ip: "2001:db8::1", want: false},
		{name: "6to4", ip: "2002:7f00:1::1", want: false},
		{name: "mapped loopback", ip: "::ffff:127.0.0.1", want: false},
		{name: "mapped private", ip: "::ffff:10.0.0.1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.ip)

			assert.Equal(t, tt.want, isPublicAddr(addr), "isPublicAddr(%s)", tt.ip)
		})
	}
}

// The fake resolver maps img.test to the test server's loopback address.
// System DNS cannot resolve img.test, so a successful fetch proves the
// downloader dials the resolver-provided IP (DNS pinning, no rebinding).
func TestFetch_DNSPinningDialsResolvedIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jpegHeader)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	_, port, err := net.SplitHostPort(serverURL.Host)
	require.NoError(t, err)

	fake := &pinResolver{addrs: []netip.Addr{netip.MustParseAddr("127.0.0.1")}}

	cfg := testConfig()
	cfg.AllowPrivate = true

	data, mime, err := newDownloader(cfg, fake).Fetch(context.Background(), "http://img.test:"+port+"/cover.jpg")

	require.NoError(t, err, "the fetch must dial the resolver-provided IP")
	assert.Equal(t, jpegHeader, data)
	assert.Equal(t, "image/jpeg", mime)
	assert.Equal(t, 1, fake.calls, "the host must be resolved exactly once per fetch")
}

// Any non-public address in the answer rejects the whole host, even next to
// a public one. The handler counts hits to prove nothing was ever dialed.
func TestFetch_MixedPublicAndPrivateRejected(t *testing.T) {
	var hits int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++

		_, _ = w.Write(jpegHeader)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	serverHost, port, err := net.SplitHostPort(serverURL.Host)
	require.NoError(t, err)

	serverIP, err := netip.ParseAddr(serverHost)
	require.NoError(t, err)

	fake := &pinResolver{addrs: []netip.Addr{netip.MustParseAddr("93.184.216.34"), serverIP}}

	cfg := testConfig()
	cfg.AllowPrivate = false

	_, _, err = newDownloader(cfg, fake).Fetch(context.Background(), "http://mixed.test:"+port+"/cover.jpg")

	require.Error(t, err, "any non-public address must reject the host")
	assert.Equal(t, 0, hits, "a rejected host must never be dialed")
}

// A failed lookup must fail the fetch even when a direct dial would succeed.
func TestFetch_ResolverErrorFailsFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jpegHeader)
	}))
	defer server.Close()

	fake := &pinResolver{err: errors.New("test dns failure")}

	cfg := testConfig()
	cfg.AllowPrivate = true

	_, _, err := newDownloader(cfg, fake).Fetch(context.Background(), server.URL+"/cover.jpg")

	require.Error(t, err, "a failed DNS resolution must fail the fetch")
}
