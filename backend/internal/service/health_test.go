package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPinger records readiness checks and returns a fixed error.
type stubPinger struct {
	calls   int
	err     error
	lastCtx context.Context
}

func (p *stubPinger) Ping(ctx context.Context) error {
	p.calls++
	p.lastCtx = ctx

	return p.err
}

func newHealthTestService(pinger Pinger) (*HealthService, *captureHandler) {
	handler := &captureHandler{}

	return NewHealthService(pinger, slog.New(handler)), handler
}

func TestHealthService_Ready_Success(t *testing.T) {
	pinger := &stubPinger{}
	svc, handler := newHealthTestService(pinger)

	require.NoError(t, svc.Ready(context.Background()))
	assert.Equal(t, 1, pinger.calls, "readiness must check the dependency exactly once")
	assert.Empty(t, handler.records, "a successful readiness check must not log")

	deadline, ok := pinger.lastCtx.Deadline()
	require.True(t, ok, "the dependency check must be bounded by a deadline")
	assert.LessOrEqual(t, time.Until(deadline), readinessPingTimeout,
		"the dependency check must not exceed the readiness timeout")
}

func TestHealthService_Ready_DependencyFailure(t *testing.T) {
	cause := errors.New("dial tcp 127.0.0.1:5432: connect: connection refused")
	pinger := &stubPinger{err: cause}
	svc, handler := newHealthTestService(pinger)

	err := svc.Ready(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"expected ErrServiceUnavailable, got %v", err)
	assert.True(t, errors.Is(err, cause), "the dependency cause must be preserved, got %v", err)
	assert.Equal(t, 1, pinger.calls)
	assert.True(t, handler.hasFC("healthService_Ready"),
		"expected a log record carrying the fc identifier")
}

func TestHealthService_Ready_PropagatesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pinger := &stubPinger{err: context.Canceled}
	svc, _ := newHealthTestService(pinger)

	err := svc.Ready(ctx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "caller cancellation must stay classifiable, got %v", err)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable))
}
