// Package errs defines the application's sentinel errors.
// Callers classify failures with errors.Is against these values.
package errs

import "errors"

var (
	ErrNewsNotFound       = errors.New("news not found")
	ErrInvalidInput       = errors.New("invalid input")
	ErrConflict           = errors.New("resource conflict")
	ErrPayloadTooLarge    = errors.New("payload too large")
	ErrServiceUnavailable = errors.New("service unavailable")
)
