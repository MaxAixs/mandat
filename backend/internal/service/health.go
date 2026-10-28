package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
)

// readinessPingTimeout bounds the database check used by the readiness probe.
const readinessPingTimeout = 2 * time.Second

// Pinger checks whether an application dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthService checks application readiness through its dependencies.
type HealthService struct {
	pinger Pinger
	logger *slog.Logger
}

// NewHealthService creates a HealthService over pinger.
func NewHealthService(pinger Pinger, logger *slog.Logger) *HealthService {
	return &HealthService{pinger: pinger, logger: logger}
}

// Ready checks that the application can serve requests.
func (s *HealthService) Ready(ctx context.Context) error {
	const fc = "healthService_Ready"

	pingCtx, cancel := context.WithTimeout(ctx, readinessPingTimeout)
	defer cancel()

	if err := s.pinger.Ping(pingCtx); err != nil {
		wrappedErr := fmt.Errorf("%s: %w", fc, errors.Join(errs.ErrServiceUnavailable, err))
		s.logger.ErrorContext(ctx, "readiness check failed", "fc", fc, "error", wrappedErr)

		return wrappedErr
	}

	return nil
}
