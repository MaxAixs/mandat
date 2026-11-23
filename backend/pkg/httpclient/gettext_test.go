package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_GetText_ReturnsBodyOn2xx pins that a 2xx page is returned as-is.
func TestClient_GetText_ReturnsBodyOn2xx(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{
			name:   "ok",
			status: http.StatusOK,
			body:   `<div class="tgme_page_extra">3 846 subscribers</div>`,
		},
		{
			name:   "upper 2xx bound",
			status: 299,
			body:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := New(server.Client(), nil)

			got, err := client.GetText(context.Background(), server.URL, 1<<20)

			require.NoError(t, err)
			assert.Equal(t, tt.body, got, "a 2xx response body must be returned as-is")
		})
	}
}

// TestClient_GetText_Non2xxIsError pins that anything outside 2xx is a failure.
func TestClient_GetText_Non2xxIsError(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{
			name:   "not found",
			status: http.StatusNotFound,
		},
		{
			name:   "server error",
			status: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("oops"))
			}))
			defer server.Close()

			client := New(server.Client(), nil)

			body, err := client.GetText(context.Background(), server.URL, 1<<20)

			require.Error(t, err, "a non-2xx status must fail the fetch")
			assert.Empty(t, body)
		})
	}
}

// TestClient_GetText_BodySizeCap pins the download bound: a body longer than
// maxBytes is rejected, while exactly maxBytes still passes.
func TestClient_GetText_BodySizeCap(t *testing.T) {
	const maxBytes = int64(8)

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "shorter body passes",
			body:    strings.Repeat("a", 7),
			wantErr: false,
		},
		{
			name:    "exactly maxBytes passes",
			body:    strings.Repeat("a", 8),
			wantErr: false,
		},
		{
			name:    "longer body rejected",
			body:    strings.Repeat("a", 9),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := New(server.Client(), nil)

			got, err := client.GetText(context.Background(), server.URL, maxBytes)

			if tt.wantErr {
				require.Error(t, err, "a body longer than maxBytes must fail")
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.body, got, "a body within maxBytes must be returned as-is")
		})
	}
}
