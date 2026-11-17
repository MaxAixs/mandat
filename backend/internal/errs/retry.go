package errs

import (
	"fmt"
	"time"
)

// RetryAfterError is a rate-limit rejection that knows when the client may retry.
// It unwraps to ErrTooManyRequests.
type RetryAfterError struct {
	Wait time.Duration
}

// NewRetryAfterError builds a rate-limit rejection with the given wait.
func NewRetryAfterError(wait time.Duration) *RetryAfterError {
	return &RetryAfterError{Wait: wait}
}

// Error implements error.
func (e *RetryAfterError) Error() string {
	return fmt.Sprintf("%s: retry after %s", ErrTooManyRequests, e.Wait)
}

// Unwrap exposes the sentinel error used for classification.
func (e *RetryAfterError) Unwrap() error { return ErrTooManyRequests }
