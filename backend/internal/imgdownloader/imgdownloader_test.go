package imgdownloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jpegHeader is the shortest byte prefix DetectContentType reports as image/jpeg.
var jpegHeader = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}

// testConfig returns a concrete client configuration for a local test server.
func testConfig() Config {
	return Config{
		Timeout:      5 * time.Second,
		MaxBytes:     1 << 20,
		MaxRedirects: 3,
	}
}

func TestFetch_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jpegHeader)
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.AllowPrivate = true

	data, mime, err := NewDownloader(cfg).Fetch(context.Background(), server.URL+"/photo")

	require.NoError(t, err)
	assert.Equal(t, jpegHeader, data)
	assert.Equal(t, "image/jpeg", mime, "the body must be sniffed, not trusted from headers")
}

func TestFetch_BlocksPrivateIP(t *testing.T) {
	cfg := testConfig()
	cfg.AllowPrivate = false

	_, _, err := NewDownloader(cfg).Fetch(context.Background(), "http://127.0.0.1:1/x")

	require.Error(t, err, "a loopback destination must be rejected before dialing")
}

func TestFetch_RejectsNonHTTP(t *testing.T) {
	cfg := testConfig()
	cfg.AllowPrivate = true

	_, _, err := NewDownloader(cfg).Fetch(context.Background(), "ftp://x/y")

	require.Error(t, err, "only http and https schemes may be fetched")
}

func TestFetch_RejectsOversized(t *testing.T) {
	cfg := testConfig()
	cfg.MaxBytes = 16 << 10
	cfg.AllowPrivate = true

	body := make([]byte, cfg.MaxBytes+1)
	copy(body, jpegHeader)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	_, _, err := NewDownloader(cfg).Fetch(context.Background(), server.URL+"/big.jpg")

	require.Error(t, err, "a body larger than MaxBytes must be rejected")
}

func TestFetch_RejectsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.AllowPrivate = true

	_, _, err := NewDownloader(cfg).Fetch(context.Background(), server.URL+"/missing.jpg")

	require.Error(t, err, "a non-2xx response must be rejected")
}
