package service

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/pkg/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultRetryDelays pins the shared retry policy the HTTP client carries
// and both rate services start with: an edit to it must not silently change
// production timings.
func TestDefaultRetryDelays(t *testing.T) {
	want := [3]time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

	assert.Equal(t, want, httpclient.DefaultDelays, "the client default must stay 5s/10s/15s")

	tests := []struct {
		name   string
		delays [3]time.Duration
	}{
		{
			name:   "exchange service",
			delays: NewExchangeService(&stubJSONClient{}, &stubExchangeStore{}, discardLogger(), nbuRatesURL).retryDelays,
		},
		{
			name:   "fuel service",
			delays: NewFuelService(&stubFuelHTTP{}, &stubFuelStore{}, discardLogger(), fuelPricesURL).retryDelays,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, want, tt.delays, "both rate services must start from the client default")
			assert.Equal(t, httpclient.DefaultDelays, tt.delays, "both rate services must share the client policy")
		})
	}
}

// assertSomeErrorLogContains fails unless one Error record's error attribute
// contains every segment, e.g. the caller fc and the fetchDay fc. It returns
// that matched error value so callers can pin the exact wrapping.
func assertSomeErrorLogContains(t *testing.T, h *captureHandler, segments ...string) string {
	t.Helper()

	var (
		values  []string
		matched string
	)

	for _, record := range h.records {
		if record.Level != slog.LevelError {
			continue
		}

		record.Attrs(func(a slog.Attr) bool {
			if a.Key != "error" {
				return true
			}

			value := a.Value.String()
			values = append(values, value)

			ok := true

			for _, segment := range segments {
				if !strings.Contains(value, segment) {
					ok = false

					break
				}
			}

			if ok && matched == "" {
				matched = value
			}

			return true
		})
	}

	assert.NotEmpty(
		t,
		matched,
		"expected one logged error to contain all of %q, got %q",
		segments,
		values,
	)

	return matched
}

// assertRetryLogs pins the retry records the HTTP client logs on behalf of a
// service: one Error record per failed retry with its attempt number and no
// service-owned fc attribute.
func assertRetryLogs(t *testing.T, h *captureHandler, message string, wantAttempts []int64) {
	t.Helper()

	var attempts []int64

	for _, record := range h.records {
		if record.Level != slog.LevelError || record.Message != message {
			continue
		}

		attempt, ok := recordAttr(record, "attempt")
		require.True(t, ok, "every retry record must carry an attempt attribute")

		attempts = append(attempts, attempt.Int64())

		errValue, ok := recordAttr(record, "error")
		require.True(t, ok, "every retry record must carry the retry error")
		assert.NotEmpty(t, errValue.String(), "the retry error attribute must not be empty")

		_, hasFC := recordAttr(record, "fc")
		assert.False(
			t,
			hasFC,
			"the retry record for attempt %d must not carry a service-owned fc attribute",
			attempt.Int64(),
		)
	}

	assert.Equal(
		t,
		wantAttempts,
		attempts,
		"expected one Error record per failed retry, numbered from the second attempt",
	)
}

// recordAttr looks up key among record.Attrs.
func recordAttr(record slog.Record, key string) (slog.Value, bool) {
	var (
		value slog.Value
		found bool
	)

	record.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			value, found = a.Value, true

			return false
		}

		return true
	})

	return value, found
}
