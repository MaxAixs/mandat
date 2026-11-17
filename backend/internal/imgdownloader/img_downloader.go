// Package imgdownloader downloads pasted image URLs for cover storage.
// Every fetched URL is untrusted input: only public http/https targets are
// allowed, so a malicious link cannot probe the private network (SSRF).
package imgdownloader

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/pkg/imagetype"
)

// DefaultTimeout bounds a single image download.
// DefaultMaxRedirects caps the redirect chain followed for one image.
const (
	DefaultTimeout      = 15 * time.Second
	DefaultMaxRedirects = 3
)

// Config tunes the fetch client.
type Config struct {
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxBytes     int64         `mapstructure:"max_bytes"`
	MaxRedirects int           `mapstructure:"max_redirects"`
	// AllowPrivate lifts the public-IP guard. Production stays false;
	// tests use it with httptest servers. Never loaded from config files.
	AllowPrivate bool `mapstructure:"-"`
}

// Downloader downloads images over HTTP.
type Downloader struct {
	http *http.Client
	max  int64
}

// resolver looks up the IP addresses of a host; *net.Resolver satisfies it.
type resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// NewDownloader builds a Downloader from cfg, pinning safe defaults for zeroes.
func NewDownloader(cfg Config) *Downloader {
	return newDownloader(cfg, net.DefaultResolver)
}

// newDownloader builds a Downloader resolving hosts through r.
func newDownloader(cfg Config, r resolver) *Downloader {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	redirects := cfg.MaxRedirects
	if redirects <= 0 {
		redirects = DefaultMaxRedirects
	}

	return &Downloader{
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: dialGuard(cfg.AllowPrivate, r),
			},
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= redirects {
					return fmt.Errorf("stopped after %d redirects", redirects)
				}

				return nil
			},
		},
		max: cfg.MaxBytes,
	}
}

// Fetch downloads rawURL and returns its body with the sniffed MIME type.
func (c *Downloader) Fetch(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: bad image url", errs.ErrInvalidInput)
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, "", fmt.Errorf("%w: image url must be http(s)", errs.ErrInvalidInput)
	}
	req.Header.Set("Accept", "image/*")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download image (%v): %w", err, errs.ErrServiceUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", fmt.Errorf("%w: image source returned status %d", errs.ErrInvalidInput, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, c.max+1))
	if err != nil {
		return nil, "", fmt.Errorf("read image body: %w", errs.ErrServiceUnavailable)
	}
	if int64(len(data)) > c.max {
		return nil, "", fmt.Errorf("%w: image exceeds %d bytes", errs.ErrPayloadTooLarge, c.max)
	}

	return data, imagetype.Sniff(data), nil
}

// dialGuard resolves the host once, rejects it when any address is not public
// (unless allowed), and dials the resolved IP itself. Dialing the hostname again
// would re-resolve it and let a rebinding DNS server swap in a private address.
func dialGuard(allowPrivate bool, r resolver) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		addrs, port, err := resolveTarget(ctx, r, addr)
		if err != nil {
			return nil, err
		}
		if !allowPrivate {
			for _, ip := range addrs {
				if !isPublicAddr(ip) {
					return nil, fmt.Errorf("%w: image host is not allowed", errs.ErrInvalidInput)
				}
			}
		}

		var dialErr error
		for _, ip := range addrs {
			conn, err := dialer.DialContext(ctx, network, netip.AddrPortFrom(ip, port).String())
			if err == nil {
				return conn, nil
			}
			dialErr = err
		}

		return nil, dialErr
	}
}

// resolveTarget splits addr and resolves its host to IPs. IP literals go through
// the resolver too; *net.Resolver returns them without a DNS query.
func resolveTarget(ctx context.Context, r resolver, addr string) ([]netip.Addr, uint16, error) {
	host, rawPort, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: bad image host", errs.ErrInvalidInput)
	}
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: bad image port", errs.ErrInvalidInput)
	}

	ips, err := r.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, 0, fmt.Errorf("%w: image host does not resolve", errs.ErrInvalidInput)
	}
	for i, ip := range ips {
		ips[i] = ip.Unmap().WithZone("")
	}

	return ips, uint16(port), nil
}

// blockedPrefixes are special-purpose ranges that are not publicly routable
// or tunnel to other address families (RFC 6890 and the IANA registries).
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fc00::/7"),
}

// isPublicAddr reports globally routable unicast addresses outside the blocked ranges.
// IPv4-mapped IPv6 addresses are judged as the IPv4 address they carry.
func isPublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}

	return true
}
