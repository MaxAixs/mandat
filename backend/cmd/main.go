// Command mandat-api runs the Mandat news API server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/maxaixs/mandat/backend/internal/config"
	"github.com/maxaixs/mandat/backend/internal/models"
	newsrepo "github.com/maxaixs/mandat/backend/internal/repository/postgres"
	redisrepo "github.com/maxaixs/mandat/backend/internal/repository/redis"
	"github.com/maxaixs/mandat/backend/internal/service"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/maxaixs/mandat/backend/internal/transport/http/middleware"
	"github.com/maxaixs/mandat/backend/migrations"
	pgdb "github.com/maxaixs/mandat/backend/pkg/database/postgres"
	"github.com/maxaixs/mandat/backend/pkg/logger"
	redisclient "github.com/maxaixs/mandat/backend/pkg/redis"
)

const configFile = "config.yaml"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(configFile, os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupCtx, cancelStartup := context.WithTimeout(ctx, cfg.HTTP.StartupTimeout)
	defer cancelStartup()

	pool, err := pgdb.NewPool(startupCtx, cfg.DB)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	if err := pgdb.Migrate(startupCtx, pool, migrations.FS); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	redisClient, err := redisclient.NewClient(startupCtx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer redisClient.Close()

	newsService := service.NewNewsService(newsrepo.NewNewsRepo(pool), log)
	authRepo := newsrepo.NewAuthRepo(pool)
	sessionStore := redisrepo.NewSessionStore(redisClient)
	authService := service.NewAuthService(authRepo, sessionStore, log, cfg.HTTP.SessionTTL)
	authHandler := handler.NewAuthHandler(
		authService,
		cfg.HTTP.SecureCookies,
		middleware.RequireAuth(authService),
		middleware.NewLoginRateLimiter().Middleware(),
	)
	newsHandler := handler.NewNewsHandler(
		newsService,
		middleware.RequireAuth(authService),
		middleware.RequireRoles(models.RoleAdmin, models.RoleModerator),
	)
	viewerMiddleware := middleware.EnsureViewer(cfg.HTTP.SecureCookies)
	publicNewsHandler := handler.NewPublicNewsHandler(newsService, viewerMiddleware)
	viewService := service.NewViewService(newsrepo.NewViewRepo(pool), log)
	viewHandler := handler.NewViewHandler(viewService, viewerMiddleware)
	healthHandler := handler.NewHealthHandler(service.NewHealthService(pool, log))

	return httptransport.New(
		cfg.HTTP,
		authHandler,
		newsHandler,
		publicNewsHandler,
		viewHandler,
		healthHandler,
	).Serve(ctx)
}
