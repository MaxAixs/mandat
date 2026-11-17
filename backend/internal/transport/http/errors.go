package httptransport

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
)

// minRetryAfterSeconds keeps Retry-After meaningful when the window is about to reset.
const minRetryAfterSeconds = 1

// ErrResp is an application error classified for the client:
// the HTTP status plus the client-facing message.
type ErrResp struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// errMapping pairs a sentinel error with its client-facing response.
type errMapping struct {
	err  error
	resp ErrResp
}

// errResps maps sentinel errors onto their client-facing responses.
// It is ordered: the first sentinel found in the error chain wins.
var errResps = []errMapping{
	{errs.ErrNewsNotFound, ErrResp{Status: http.StatusNotFound, Message: "News article not found"}},
	{errs.ErrPayloadTooLarge, ErrResp{Status: http.StatusRequestEntityTooLarge, Message: "Request payload too large"}},
	{errs.ErrInvalidInput, ErrResp{Status: http.StatusBadRequest, Message: "Invalid input"}},
	{errs.ErrConflict, ErrResp{Status: http.StatusConflict, Message: "Resource conflict"}},
	{errs.ErrUnauthenticated, ErrResp{Status: http.StatusUnauthorized, Message: "Authentication required"}},
	{errs.ErrUnauthorized, ErrResp{Status: http.StatusUnauthorized, Message: "Invalid login or password"}},
	{errs.ErrForbidden, ErrResp{Status: http.StatusForbidden, Message: "Insufficient permissions"}},
	{errs.ErrUntrustedOrigin, ErrResp{Status: http.StatusForbidden, Message: "Untrusted request origin"}},
	{errs.ErrTooManyRequests, ErrResp{Status: http.StatusTooManyRequests, Message: "Too many requests"}},
	{errs.ErrServiceUnavailable, ErrResp{Status: http.StatusServiceUnavailable, Message: "Service unavailable"}},
}

// respInternal is the fallback response for unexpected errors.
var respInternal = ErrResp{Status: http.StatusInternalServerError, Message: "Internal server error"}

// WriteError maps an application error onto a JSON error response.
// A rate-limit rejection that knows its wait also sets Retry-After.
func WriteError(c *gin.Context, err error) {
	if retryErr, ok := errors.AsType[*errs.RetryAfterError](err); ok {
		c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(retryErr)))
	}

	resp := errResp(err)

	c.AbortWithStatusJSON(resp.Status, gin.H{"error": resp})
}

// errResp classifies an application error into an ErrResp.
func errResp(err error) ErrResp {
	if validationErr, ok := errors.AsType[*errs.ValidationError](err); ok {
		return ErrResp{Status: http.StatusBadRequest, Message: validationErr.Reason}
	}

	for _, mapping := range errResps {
		if errors.Is(err, mapping.err) {
			return mapping.resp
		}
	}

	return respInternal
}

// retryAfterSeconds rounds the wait up to whole seconds, at least minRetryAfterSeconds.
func retryAfterSeconds(retryErr *errs.RetryAfterError) int {
	return max(int(math.Ceil(retryErr.Wait.Seconds())), minRetryAfterSeconds)
}
