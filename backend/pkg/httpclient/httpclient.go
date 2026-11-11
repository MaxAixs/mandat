// Package httpclient provides one shared HTTP client for the whole
// application plus a JSON client for trusted outbound calls.
// The shared client owns the connection pool; services declare the
// narrow interface they need and the client implements it.
// Cases with special security needs (SSRF guards, body limits)
// keep their own clients.
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NewSharedClient creates one client for the whole application with
// the given per-request timeout. Sharing a client reuses connections;
// build it once in main and inject it everywhere instead of creating
// a client per use case.
func NewSharedClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// Client is a JSON API client over a shared HTTP client.
type Client struct {
	http *http.Client
}

// New creates a Client over httpClient.
func New(httpClient *http.Client) *Client {
	return &Client{http: httpClient}
}

// GetJSON performs a GET request to url and decodes a 2xx JSON response
// into out.
func (c *Client) GetJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("httpclient_GetJSON: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("httpclient_GetJSON: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("httpclient_GetJSON: unexpected status %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("httpclient_GetJSON: decode response: %w", err)
	}

	return nil
}
