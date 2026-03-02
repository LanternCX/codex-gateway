package cli

import (
	"context"
	"fmt"
	"os"

	"codex-gateway/internal/auth"
	"codex-gateway/internal/config"
	"codex-gateway/internal/logging"
	"codex-gateway/internal/oauth"
	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "OAuth authentication commands",
	}

	auth.AddCommand(newAuthLoginCommand())
	return auth
}

func newAuthLoginCommand() *cobra.Command {
	var workdir string
	var configFile string
	var mode string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Run interactive OAuth login",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(cmd.Context(), workdir, configFile, mode)
		},
	}

	cmd.Flags().StringVar(&workdir, "workdir", ".", "Runtime working directory")
	cmd.Flags().StringVar(&configFile, "config", "config.yaml", "Config file path (must be inside workdir)")
	cmd.Flags().StringVar(&mode, "mode", "callback", "OAuth login mode: callback or device")

	return cmd
}

func runAuthLogin(ctx context.Context, workdir, configFile, mode string) error {
	paths, err := resolveRuntimePaths(workdir, configFile)
	if err != nil {
		return err
	}

	cfg, err := config.Load(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	rootLogger, err := logging.New(cfg.Logging.Level, cfg.Logging.Format, os.Stdout)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	logger := rootLogger.With("component", "auth")

	store := auth.NewFileStore(paths.TokenPath)

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
	})

	var token auth.Token
	logger.InfoContext(ctx, "starting oauth login", "mode", mode)
	switch mode {
	case "callback":
		token, err = oauthClient.AuthenticateWithCallback(ctx, os.Stdout)
	case "device":
		token, err = oauthClient.Authenticate(ctx, os.Stdout)
	default:
		return fmt.Errorf("unsupported login mode %q (expected callback or device)", mode)
	}

	if err != nil {
		logger.ErrorContext(ctx, "oauth login failed", "mode", mode, "error", err)
		return fmt.Errorf("oauth login failed: %w", err)
	}

	if token.RefreshToken == "" {
		return fmt.Errorf("oauth login succeeded but refresh_token is empty")
	}

	if err := store.Save(token); err != nil {
		logger.ErrorContext(ctx, "save oauth token failed", "token_path", paths.TokenPath, "error", err)
		return fmt.Errorf("save token: %w", err)
	}

	logger.InfoContext(ctx, "oauth login succeeded", "mode", mode, "token_path", paths.TokenPath)
	fmt.Fprintf(os.Stdout, "Login successful. Token saved to %s\n", paths.TokenPath)
	return nil
}
