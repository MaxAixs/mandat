// Package httptransport wires the HTTP routes of the API and runs the server.
package httptransport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Server owns the http server, its gin router, and the graceful
// shutdown policy.
type Server struct {
	server *http.Server
	router *gin.Engine

	shutdownTimeout time.Duration
}

// New assembles the API from the config: it builds the gin router
// with the global middleware, binds every given registrar through its
// Register method, and wires the router into the http server.
// middlewares run on every route after CORS. The router always runs in
// release mode; an admin API has nothing to debug.
func New(cfg HTTPConfig, middlewares []gin.HandlerFunc, registrars ...Registrar) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Port),
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		router:          gin.New(),
		shutdownTimeout: cfg.ShutdownTimeout,
	}

	// Client IP headers are honoured only from trusted proxies;
	// with none configured the client IP is the TCP peer address.
	if err := s.router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	// Assigned unconditionally: an empty list honours no header at all,
	// rather than falling back to gin's X-Forwarded-For/X-Real-IP defaults.
	s.router.RemoteIPHeaders = cfg.ClientIPHeaders

	s.router.Use(gin.Recovery())
	s.router.Use(securityHeaders(cfg.SecureCookies))
	s.router.Use(limitBodySize(defaultMaxBodyBytes))

	if len(cfg.AllowedOrigins) > 0 {
		s.router.Use(corsMiddleware(cfg.AllowedOrigins))
	}

	// After CORS, so a rejected cross-site write still carries CORS headers.
	s.router.Use(originGuard(cfg.AllowedOrigins))

	// After CORS, so a rejection still carries the CORS headers the browser needs.
	s.router.Use(middlewares...)

	api := s.router.Group("")
	for _, registrar := range registrars {
		if registrar != nil {
			registrar.Register(api)
		}
	}

	s.server.Handler = s.router

	return s, nil
}

// Handler exposes the assembled router as an http.Handler, so the
// external test package can drive requests without opening the port.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Serve runs the server until ctx is done, then shuts it down gracefully.
// A listen failure is reported as the error; ctx cancellation is not.
func (s *Server) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}

		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	return s.Shutdown()
}

// Shutdown stops accepting new connections and waits for the in-flight
// requests to finish, up to the configured shutdown timeout.
func (s *Server) Shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	return nil
}
