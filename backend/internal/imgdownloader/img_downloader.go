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
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
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

// NewDownloader builds a Downloader from cfg, pinning safe defaults for zeroes.
func NewDownloader(cfg Config) *Downloader {
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
				DialContext: dialGuard(cfg.AllowPrivate),
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

	return data, sniffMIME(data), nil
}

// sniffMIME reports the content type from the leading body bytes.
func sniffMIME(data []byte) string {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}

	return http.DetectContentType(head)
}

// dialGuard rejects non-public destination IPs unless allowed.
func dialGuard(allowPrivate bool) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if !allowPrivate {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("%w: bad image host", errs.ErrInvalidInput)
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("%w: image host does not resolve", errs.ErrInvalidInput)
			}
			for _, ip := range ips {
				if !isPublicIP(ip) {
					return nil, fmt.Errorf("%w: image host is not allowed", errs.ErrInvalidInput)
				}
			}
		}

		return dialer.DialContext(ctx, network, addr)
	}
}

// isPublicIP reports globally routable unicast addresses.
func isPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate()
}
