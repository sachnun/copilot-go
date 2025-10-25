package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"internal/logger"
	"internal/paths"
	"internal/server"
	"internal/services/copilot"
	"internal/services/vscode"
	"internal/state"
	"internal/token"
)

type RunServerOptions struct {
	Port             int
	Verbose          bool
	AccountType      string
	Manual           bool
	RateLimitSeconds *int
	RateLimitWait    bool
	GitHubToken      string
	ShowToken        bool
	ProxyEnv         bool
}

func RunServer(ctx context.Context, opts RunServerOptions) error {
	if opts.Verbose {
		logger.SetLevel(logger.LevelDebug)
		logger.Info("Verbose logging enabled")
	} else {
		logger.SetLevel(logger.LevelInfo)
	}

	if opts.ProxyEnv {
		logger.Info("Using proxy configuration from environment")
	}

	state.Shared.Update(func(st *state.State) {
		st.AccountType = opts.AccountType
		st.ManualApprove = opts.Manual
		st.RateLimitWait = opts.RateLimitWait
		st.ShowToken = opts.ShowToken
		if opts.RateLimitSeconds != nil {
			v := *opts.RateLimitSeconds
			st.RateLimitSeconds = &v
		} else {
			st.RateLimitSeconds = nil
		}
	})

	if err := paths.EnsurePaths(paths.Default); err != nil {
		return err
	}

	client := &http.Client{}

	version := vscode.GetVersion(ctx, client)
	state.Shared.Update(func(st *state.State) {
		st.VSCodeVersion = version
	})
	logger.Info("Using VSCode version: %s", version)

	if opts.GitHubToken != "" {
		state.Shared.Update(func(st *state.State) {
			st.GitHubToken = opts.GitHubToken
		})
		logger.Info("Using provided GitHub token")
	} else {
		if err := token.SetupGitHubToken(ctx, state.Shared, paths.Default, token.SetupGitHubTokenOptions{}, client); err != nil {
			return err
		}
	}

	cancelRefresh, err := token.SetupCopilotToken(ctx, state.Shared, client)
	if err != nil {
		return err
	}
	defer cancelRefresh()

	models, err := copilot.GetModels(ctx, state.Shared, client)
	if err != nil {
		return err
	}
	state.Shared.Update(func(st *state.State) {
		st.Models = models
	})

	logger.Info("Available models:")
	for _, model := range models.Data {
		logger.Info("- %s", model.ID)
	}

	now := time.Now().UnixMilli()
	state.Shared.Update(func(st *state.State) {
		st.ServerStartUnixMs = &now
	})

	srv := server.New(state.Shared, client)
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", opts.Port),
		Handler: srv.Handler(),
	}

	logger.Info("🌐 Usage Viewer: https://ericc-ch.github.io/copilot-api?endpoint=http://localhost:%d/usage", opts.Port)

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		return nil
	}
}
