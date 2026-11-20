package httptransport

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteError_MapsAdFormatNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "direct",
			err:  errs.ErrAdFormatNotFound,
		},
		{
			name: "wrapped",
			err:  fmt.Errorf("adFormatService_GetByID: %w", errs.ErrAdFormatNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := writeErrorResponse(t, tt.err)

			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			payload := decodeWriteError(t, rec)
			assert.Equal(t, http.StatusNotFound, payload.Error.Status)
			assert.Equal(t, "Ad format not found", payload.Error.Message)
		})
	}
}
