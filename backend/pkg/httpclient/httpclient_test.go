package httpclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastDelays keeps the retry path instant while still exercising every retry.
var fastDelays = [3]time.Duration{
	time.Millisecond,
	2 * time.Millisecond,
	3 * time.Millisecond,
}

// slowDelays lets a cancellation test prove the helper aborts instead of
// sleeping through the configured pauses.
var slowDelays = [3]time.Duration{
	10 * time.Second,
	10 * time.Second,
	10 * time.Second,
}

// recordHandler records slog records for assertions.
// It is the pkg-local counterpart of the service test captureHandler,
// which is not importable across packages.
type recordHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, r)

	return nil
}

func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordHandler) WithGroup(string) slog.Handler { return h }

// snapshot returns a copy of the records logged so far.
func (h *recordHandler) snapshot() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	records := make([]slog.Record, len(h.records))
	copy(records, h.records)

	return records
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

// TestFetchWithRetries_AttemptBudget pins the attempt budget: one immediate
// call plus one call per configured delay, and no log for a failed initial
// attempt or a successful one.
func TestFetchWithRetries_AttemptBudget(t *testing.T) {
	upstreamErr := errors.New("upstream unavailable")

	tests := []struct {
		name      string
		failCalls int
		wantCalls int
		wantLogs  int
		wantErr   bool
	}{
		{
			name:      "success on first attempt",
			failCalls: 0,
			wantCalls: 1,
			wantLogs:  0,
		},
		{
			// The initial attempt is never logged: retry 1 (attempt 2) fails and
			// logs, retry 2 (attempt 3) succeeds and returns immediately.
			name:      "two failures then success",
			failCalls: 2,
			wantCalls: 3,
			wantLogs:  1,
		},
		{
			name:      "all attempts fail",
			failCalls: 4,
			wantCalls: 4,
			wantLogs:  3,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &recordHandler{}
			client := New(&http.Client{}, slog.New(handler))

			calls := 0
			fn := func(context.Context) error {
				calls++

				if calls <= tt.failCalls {
					return fmt.Errorf("attempt %d: %w", calls, upstreamErr)
				}

				return nil
			}

			err := client.FetchWithRetries(
				context.Background(),
				fastDelays,
				"upstream fetch retry failed",
				fn,
			)

			assert.Equal(t, tt.wantCalls, calls, "one attempt plus one attempt per delay")
			assert.Len(t, handler.snapshot(), tt.wantLogs, "only failed retries log")

			if !tt.wantErr {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.True(t, errors.Is(err, upstreamErr), "expected the last error, got %v", err)
			assert.Equal(
				t,
				"attempt 4: upstream unavailable",
				err.Error(),
				"the final failure must be the last error unwrapped",
			)
		})
	}
}

// TestFetchWithRetries_ZeroDelaysRetryImmediately pins the zero-value delay
// contract: an all-zero delays array means three immediate retries, not "no
// retry", which is the documented footgun of the fixed-size array.
func TestFetchWithRetries_ZeroDelaysRetryImmediately(t *testing.T) {
	upstreamErr := errors.New("upstream unavailable")
	handler := &recordHandler{}
	client := New(&http.Client{}, slog.New(handler))

	calls := 0
	fn := func(context.Context) error {
		calls++

		return fmt.Errorf("attempt %d: %w", calls, upstreamErr)
	}

	start := time.Now()
	err := client.FetchWithRetries(context.Background(), [3]time.Duration{}, "upstream fetch retry failed", fn)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, 4, calls, "zero delays must still spend the full attempt budget")
	assert.Len(t, handler.snapshot(), 3, "zero delays must still log one record per failed retry")
	assert.Less(t, elapsed, time.Second, "zero delays must retry immediately")
}

// TestFetchWithRetries_CanceledContextAborts pins that a canceled context stops
// the helper without waiting out the retry delays and returns ctx.Err()
// unwrapped.
func TestFetchWithRetries_CanceledContextAborts(t *testing.T) {
	initialErr := errors.New("upstream unavailable")

	tests := []struct {
		name         string
		cancelBefore bool
	}{
		{
			name:         "canceled before the first attempt",
			cancelBefore: true,
		},
		{
			// The first attempt fails, then the wait must observe ctx.Done and
			// abort instead of calling fn again or returning the attempt error.
			name:         "canceled while waiting to retry",
			cancelBefore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &recordHandler{}
			client := New(&http.Client{}, slog.New(handler))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.cancelBefore {
				cancel()
			}

			calls := 0
			fn := func(fnCtx context.Context) error {
				calls++

				if !tt.cancelBefore {
					cancel()

					return initialErr
				}

				return fnCtx.Err()
			}

			start := time.Now()
			err := client.FetchWithRetries(ctx, slowDelays, "upstream fetch retry failed", fn)
			elapsed := time.Since(start)

			require.Error(t, err)
			assert.True(t, errors.Is(err, context.Canceled), "expected context.Canceled, got %v", err)
			assert.Equal(t, "context canceled", err.Error(), "the aborted error must be returned unwrapped")
			assert.False(t, errors.Is(err, initialErr), "aborting must not return the failed attempt error")
			assert.Less(t, elapsed, 5*time.Second, "a canceled context must abort instead of sleeping through the delays")
			assert.Equal(t, 1, calls, "a canceled context must abort after the first attempt")
			assert.Empty(t, handler.snapshot(), "an aborted wait was never attempted, so it must not log")
		})
	}
}

// TestFetchWithRetries_WaitsBetweenAttempts pins that every delay is honored
// before its retry instead of retrying all at once.
func TestFetchWithRetries_WaitsBetweenAttempts(t *testing.T) {
	delays := [3]time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
	}
	waitTotal := delays[0] + delays[1] + delays[2]

	handler := &recordHandler{}
	client := New(&http.Client{}, slog.New(handler))

	calls := 0
	fn := func(context.Context) error {
		calls++

		return errors.New("upstream unavailable")
	}

	start := time.Now()
	err := client.FetchWithRetries(context.Background(), delays, "upstream fetch retry failed", fn)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, 4, calls, "one attempt plus three retries must all run")
	assert.Len(t, handler.snapshot(), 3, "each failed retry must log exactly once")
	assert.GreaterOrEqual(t, elapsed, waitTotal, "every retry must wait for its configured delay")
}

// TestFetchWithRetries_LogsFailedRetries pins the log shape the client owns:
// one Error record per failed retry, the caller's message, the attempt number
// and the retry error.
func TestFetchWithRetries_LogsFailedRetries(t *testing.T) {
	const errorContext = "exchange fetch retry failed"

	upstreamErr := errors.New("upstream unavailable")
	handler := &recordHandler{}
	client := New(&http.Client{}, slog.New(handler))

	calls := 0
	fn := func(context.Context) error {
		calls++

		return fmt.Errorf("attempt %d: %w", calls, upstreamErr)
	}

	err := client.FetchWithRetries(context.Background(), fastDelays, errorContext, fn)

	require.Error(t, err)
	require.Equal(t, 4, calls, "a total failure must spend the full attempt budget")

	records := handler.snapshot()
	require.Len(t, records, 3, "one Error record per failed retry")

	var attempts []int64

	for i, record := range records {
		assert.Equal(t, slog.LevelError, record.Level, "retry %d must log at Error", i+1)
		assert.Equal(t, errorContext, record.Message, "the message must be the caller's error context")

		attempt, ok := recordAttr(record, "attempt")
		require.True(t, ok, "retry %d must carry an attempt attribute", i+1)
		attempts = append(attempts, attempt.Int64())

		errValue, ok := recordAttr(record, "error")
		require.True(t, ok, "retry %d must carry the retry error", i+1)
		assert.Contains(
			t,
			errValue.String(),
			upstreamErr.Error(),
			"retry %d must log its own retry error",
			i+1,
		)
	}

	assert.Equal(t, []int64{2, 3, 4}, attempts, "failed retries are numbered from the second attempt")
}

// TestFetchWithRetries_NilLoggerDefaults pins that a nil logger must not panic
// on the retry error path: New defaults it to slog.Default, so a total failure
// still returns the last error.
func TestFetchWithRetries_NilLoggerDefaults(t *testing.T) {
	upstreamErr := errors.New("upstream unavailable")
	client := New(&http.Client{}, nil)

	calls := 0
	fn := func(context.Context) error {
		calls++

		return fmt.Errorf("attempt %d: %w", calls, upstreamErr)
	}

	var err error

	require.NotPanics(
		t,
		func() {
			err = client.FetchWithRetries(context.Background(), fastDelays, "upstream fetch retry failed", fn)
		},
		"a nil logger must default to slog.Default instead of panicking on the retry error path",
	)

	require.Error(t, err)
	assert.True(t, errors.Is(err, upstreamErr), "expected the last error, got %v", err)
	assert.Equal(t, "attempt 4: upstream unavailable", err.Error(), "a total failure must return the last attempt error")
	assert.Equal(t, 4, calls, "a nil logger must not change the attempt budget")
}

// TestFetchWithRetries_SuccessLogsNothing pins that a successful first attempt
// neither retries nor logs.
func TestFetchWithRetries_SuccessLogsNothing(t *testing.T) {
	handler := &recordHandler{}
	client := New(&http.Client{}, slog.New(handler))

	calls := 0
	fn := func(context.Context) error {
		calls++

		return nil
	}

	err := client.FetchWithRetries(context.Background(), slowDelays, "exchange fetch retry failed", fn)

	require.NoError(t, err)
	assert.Equal(t, 1, calls, "a successful first attempt must not retry")
	assert.Empty(t, handler.snapshot(), "a successful attempt must not log anything")
}
