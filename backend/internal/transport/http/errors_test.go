package httptransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeErrorResponse runs WriteError through a gin test context and returns
// the recorded response.
func writeErrorResponse(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	WriteError(c, err)

	return rec
}

// errorPayload mirrors the wire shape of every error response.
type errorPayload struct {
	Error struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeWriteError(t *testing.T, rec *httptest.ResponseRecorder) errorPayload {
	t.Helper()

	var payload errorPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))

	return payload
}

func TestWriteError_MapsKnownErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatus     int
		wantMessage    string
		wantRetryAfter string
	}{
		{
			name:           "rate limit without wait",
			err:            errs.ErrTooManyRequests,
			wantStatus:     http.StatusTooManyRequests,
			wantMessage:    "Too many requests",
			wantRetryAfter: "",
		},
		{
			name:           "retry after rounds up",
			err:            errs.NewRetryAfterError(1500 * time.Millisecond),
			wantStatus:     http.StatusTooManyRequests,
			wantMessage:    "Too many requests",
			wantRetryAfter: "2",
		},
		{
			name:           "wrapped retry after keeps header",
			err:            fmt.Errorf("x: %w", errs.NewRetryAfterError(1500*time.Millisecond)),
			wantStatus:     http.StatusTooManyRequests,
			wantMessage:    "Too many requests",
			wantRetryAfter: "2",
		},
		{
			name:           "zero wait clamps to one",
			err:            errs.NewRetryAfterError(0),
			wantStatus:     http.StatusTooManyRequests,
			wantMessage:    "Too many requests",
			wantRetryAfter: "1",
		},
		{
			name:           "whole seconds pass through",
			err:            errs.NewRetryAfterError(30 * time.Second),
			wantStatus:     http.StatusTooManyRequests,
			wantMessage:    "Too many requests",
			wantRetryAfter: "30",
		},
		{
			name:           "unauthenticated asks to log in",
			err:            errs.ErrUnauthenticated,
			wantStatus:     http.StatusUnauthorized,
			wantMessage:    "Authentication required",
			wantRetryAfter: "",
		},
		{
			name:           "failed login",
			err:            errs.ErrUnauthorized,
			wantStatus:     http.StatusUnauthorized,
			wantMessage:    "Invalid login or password",
			wantRetryAfter: "",
		},
		{
			name:           "forbidden staff action",
			err:            errs.ErrForbidden,
			wantStatus:     http.StatusForbidden,
			wantMessage:    "Insufficient permissions",
			wantRetryAfter: "",
		},
		{
			name:           "untrusted request origin",
			err:            errs.ErrUntrustedOrigin,
			wantStatus:     http.StatusForbidden,
			wantMessage:    "Untrusted request origin",
			wantRetryAfter: "",
		},
		{
			name:           "validation error keeps reason",
			err:            errs.NewValidationError("title", "title is required"),
			wantStatus:     http.StatusBadRequest,
			wantMessage:    "title is required",
			wantRetryAfter: "",
		},
		{
			name:           "unknown error stays generic",
			err:            errors.New("boom"),
			wantStatus:     http.StatusInternalServerError,
			wantMessage:    "Internal server error",
			wantRetryAfter: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := writeErrorResponse(t, tt.err)

			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
			payload := decodeWriteError(t, rec)
			assert.Equal(t, tt.wantStatus, payload.Error.Status)
			assert.Equal(t, tt.wantMessage, payload.Error.Message)
			assert.Equal(t, tt.wantRetryAfter, rec.Header().Get("Retry-After"))
		})
	}
}

func TestWriteError_DeterministicOnJoinedErrors(t *testing.T) {
	joined := errors.Join(errs.ErrInvalidInput, errs.ErrPayloadTooLarge)

	first := writeErrorResponse(t, joined)
	want := first.Code

	for i := 0; i < 200; i++ {
		rec := writeErrorResponse(t, joined)
		require.Equal(t, want, rec.Code,
			"iteration %d: a joined error must map to one stable status", i)
	}
}

func TestWriteError_BodyShape(t *testing.T) {
	rec := writeErrorResponse(t, errs.ErrForbidden)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	var raw map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.Contains(t, raw, "error")
	assert.Equal(t, float64(http.StatusForbidden), raw["error"]["status"])
	assert.Equal(t, "Insufficient permissions", raw["error"]["message"])
}
