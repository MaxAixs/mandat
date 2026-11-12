// Package httpclient provides a JSON client for trusted outbound calls
// over a shared HTTP client. Services declare the narrow interface they
// need and the client implements it. Cases with special security needs
// (SSRF guards, body limits) keep their own clients.
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// DefaultDelays spaces the three retries of a fetch.
var DefaultDelays = [3]time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

// Client is a JSON API client over a shared HTTP client.
type Client struct {
	http   *http.Client
	logger *slog.Logger
}

// New creates a Client over httpClient. A nil logger defaults to slog.Default.
func New(httpClient *http.Client, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{http: httpClient, logger: logger}
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

// FetchWithRetries runs fn once, then retries it once per delay.
// Aborted waits return ctx.Err(), discarding the attempt error. Failures
// report the 1-based failed attempt (2,3,4) via the log; fn runs synchronously
// in the caller's goroutine, so closures capturing results are safe.
// Errors return unwrapped for the caller to wrap.
func (c *Client) FetchWithRetries(
	ctx context.Context,
	delays [3]time.Duration,
	errorContext string,
	fn func(context.Context) error,
) error {
	err := fn(ctx)
	if err == nil {
		return nil
	}

	for i, pause := range delays {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pause):
		}

		err = fn(ctx)
		if err == nil {
			return nil
		}

		c.logger.ErrorContext(ctx, errorContext, "attempt", i+2, "error", err)
	}

	return err
}
