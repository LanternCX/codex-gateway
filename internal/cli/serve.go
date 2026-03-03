package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codex-gateway/internal/auth"
	"codex-gateway/internal/config"
	"codex-gateway/internal/oauth"
	"codex-gateway/internal/server"
	"codex-gateway/internal/upstream"
	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	var workdir string
	var configFile string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run gateway HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context(), workdir, configFile)
		},
	}

	cmd.Flags().StringVar(&workdir, "workdir", ".", "Runtime working directory")
	cmd.Flags().StringVar(&configFile, "config", "config.yaml", "Config file path (must be inside workdir)")

	return cmd
}

func runServe(ctx context.Context, workdir, configFile string) error {
	paths, err := resolveRuntimePaths(workdir, configFile)
	if err != nil {
		return err
	}

	cfg, err := config.Load(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	rootLogger, err := newRootLogger(cfg.Logging, paths.Workdir)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	logger := rootLogger.With("component", "cli")
	serverLogger := rootLogger.With("component", "server")
	authLogger := rootLogger.With("component", "auth")
	upstreamLogger := rootLogger.With("component", "upstream")

	store := auth.NewFileStore(paths.TokenPath)
	oauthHTTPClient, err := newOAuthHTTPClient(cfg.Network.ProxyURL)
	if err != nil {
		return fmt.Errorf("build oauth http client: %w", err)
	}

	oauthClient := oauth.NewClient(oauth.Config{
		ClientID:                    cfg.OAuth.ClientID,
		ClientSecret:                cfg.OAuth.ClientSecret,
		AuthorizeEndpoint:           cfg.OAuth.AuthorizeEndpoint,
		DeviceAuthorizationEndpoint: cfg.OAuth.DeviceAuthorizationEndpoint,
		TokenEndpoint:               cfg.OAuth.TokenEndpoint,
		RedirectHost:                cfg.OAuth.RedirectHost,
		RedirectPort:                cfg.OAuth.RedirectPort,
		RedirectPath:                cfg.OAuth.RedirectPath,
		Originator:                  cfg.OAuth.Originator,
		Scopes:                      cfg.OAuth.Scopes,
		Audience:                    cfg.OAuth.Audience,
	}, oauth.WithHTTPClient(oauthHTTPClient))

	manager := auth.NewManager(store, func(ctx context.Context, in auth.Token) (auth.Token, error) {
		refreshed, err := oauthClient.RefreshToken(ctx, in.RefreshToken)
		if err != nil {
			return auth.Token{}, err
		}

		if refreshed.RefreshToken == "" {
			refreshed.RefreshToken = in.RefreshToken
		}

		return refreshed, nil
	}, auth.WithLogger(authLogger))

	upstreamBaseURL := cfg.Upstream.BaseURL
	if cfg.Upstream.Mode == "codex_oauth" {
		upstreamBaseURL = cfg.Upstream.CodexBaseURL
	}

	upstreamHTTPClient, err := newUpstreamHTTPClient(cfg.Upstream.TimeoutSeconds, cfg.Network.ProxyURL)
	if err != nil {
		return fmt.Errorf("build upstream http client: %w", err)
	}

	upstreamClient := upstream.NewClient(
		upstreamBaseURL,
		time.Duration(cfg.Upstream.TimeoutSeconds)*time.Second,
		upstream.WithHTTPClient(upstreamHTTPClient),
		upstream.WithLogger(upstreamLogger),
	)

	handler := server.New(buildServerDependencies(cfg, manager, upstreamClient, serverLogger))

	httpServer := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: handler,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	logger.InfoContext(ctx, "gateway server starting", "listen", cfg.Server.Listen, "workdir", paths.Workdir, "upstream_mode", cfg.Upstream.Mode)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			logger.InfoContext(ctx, "gateway server stopped")
			return nil
		}
		logger.ErrorContext(ctx, "gateway server exited with error", "error", err)
		return err
	case <-ctx.Done():
		logger.InfoContext(ctx, "gateway server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.ErrorContext(ctx, "gateway server shutdown failed", "error", err)
			return fmt.Errorf("shutdown server: %w", err)
		}
		logger.InfoContext(ctx, "gateway server shutdown completed")
		return nil
	}
}

func buildServerDependencies(cfg config.Config, tokenProvider server.TokenProvider, upstreamClient server.UpstreamClient, logger *slog.Logger) server.Dependencies {
	return server.Dependencies{
		FixedAPIKey:         cfg.Auth.DownstreamAPIKey,
		ModelsPath:          cfg.Upstream.ModelsPath,
		ChatCompletionsPath: cfg.Upstream.ChatCompletionsPath,
		ResponsesPath:       cfg.Upstream.ResponsesPath,
		CodexCompat:         cfg.Upstream.Mode == "codex_oauth",
		CodexResponsesPath:  cfg.Upstream.CodexResponsesPath,
		CodexOriginator:     cfg.OAuth.Originator,
		Logger:              logger,
		TokenProvider:       tokenProvider,
		UpstreamClient:      upstreamClient,
	}
}
