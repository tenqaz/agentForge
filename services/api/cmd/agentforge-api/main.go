package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/config"
	"agentforge.local/services/api/internal/db"
	httpapi "agentforge.local/services/api/internal/http"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/templates"
	"agentforge.local/services/api/internal/weixin"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slog.Info("Starting AgentForge API", "http_addr", cfg.HTTPAddr, "data_dir", cfg.DataDir)

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	database, err := db.Open(ctx, cfg.SQLitePath)
	if err != nil {
		return err
	}
	defer database.Close()

	migrationsDir, err := resolveMigrationsDir()
	if err != nil {
		return err
	}
	if err := db.Migrate(ctx, database, migrationsDir); err != nil {
		return err
	}

	authRepo := auth.NewRepository(database)
	if err := authRepo.EnsureDefaultAdmin(ctx); err != nil {
		return fmt.Errorf("ensure default admin: %w", err)
	}
	sessionManager := auth.NewSessionManager(cfg.SessionSecret, false)
	templateRepo := templates.NewRepository(database)
	templateStore := templates.NewFileStore(cfg.DataDir)
	runtimeJobs := jobs.NewRuntimeRepository(database)
	agentRepo := agents.NewRepository(database)
	templateService := templates.NewService(templateRepo, templateStore, agentRepo)
	runner := runtime.NewDockerRunner(cfg.DockerBin)
	agentService := agents.NewService(database, agentRepo, runtimeJobs, runner, cfg.DataDir)
	channelRepo := channels.NewRepository(database)
	channelService := channels.NewService(database, channelRepo)
	channelJobs := jobs.NewChannelRepository(database)
	templateLoader := templateService
	runtimeWorker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:       database,
		RuntimeJobs:    runtimeJobs,
		Runner:         runner,
		TemplateLoader: templateLoader,
		HermesImage:    cfg.HermesImage,
		HermesMemory:   cfg.HermesMemory,
		HermesCPUs:     cfg.HermesCPUs,
	})
	weixinClient := weixin.NewClient(cfg.WeixinBaseURL, nil)
	channelWorker := jobs.NewChannelWorker(jobs.ChannelWorkerDependencies{
		Database:     database,
		ChannelJobs:  channelJobs,
		Channels:     channelRepo,
		WeixinClient: weixinClient,
		Runner:       runner,
	})
	supervisor := jobs.NewSupervisor(jobs.SupervisorDependencies{
		RuntimeJobs:   runtimeJobs,
		ChannelJobs:   channelJobs,
		RuntimeWorker: runtimeWorker,
		ChannelWorker: channelWorker,
	})

	router := httpapi.NewRouter(httpapi.Dependencies{
		AuthRepository:       authRepo,
		SessionManager:       sessionManager,
		TemplateService:      templateService,
		AgentService:         agentService,
		RuntimeJobRepository: runtimeJobs,
		ChannelService:       channelService,
		ChannelRepository:    channelRepo,
		ChannelJobRepository: channelJobs,
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
	}

	errCh := make(chan error, 2)
	go func() {
		slog.Info("Starting job supervisor")
		errCh <- supervisor.Run(ctx)
	}()
	go func() {
		slog.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		slog.Info("Shutdown signal received, gracefully stopping...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		slog.Error("Server error", "error", err)
		return fmt.Errorf("listen and serve: %w", err)
	}
}

func resolveMigrationsDir() (string, error) {
	candidates := []string{
		"migrations",
		filepath.Join("..", "..", "migrations"),
		filepath.Join("services", "api", "migrations"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("migrations directory not found")
}
