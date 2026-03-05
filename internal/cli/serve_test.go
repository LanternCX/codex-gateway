package cli

import (
	"testing"

	"codex-gateway/internal/config"
)

func TestBuildServerDependencies_PropagatesResponsesPath(t *testing.T) {
	cfg := config.Config{
		Auth: config.AuthConfig{
			DownstreamAPIKey: "fixed-key",
		},
		OAuth: config.OAuthConfig{
			Originator: "opencode",
		},
		Upstream: config.UpstreamConfig{
			Mode:                "openai_api",
			ModelsPath:          "/custom/models",
			ChatCompletionsPath: "/custom/chat/completions",
			ResponsesPath:       "/custom/responses",
			CodexResponsesPath:  "/backend-api/codex/responses",
		},
	}

	deps := buildServerDependencies(cfg, nil, nil, nil)

	if deps.ResponsesPath != cfg.Upstream.ResponsesPath {
		t.Fatalf("unexpected responses path: got %q want %q", deps.ResponsesPath, cfg.Upstream.ResponsesPath)
	}
}
