package app

import (
	"context"
	"net/http"

	"internal/logger"
	"internal/paths"
	"internal/state"
	"internal/token"
)

type RunAuthOptions struct {
	Verbose   bool
	ShowToken bool
}

func RunAuth(ctx context.Context, opts RunAuthOptions) error {
	if opts.Verbose {
		logger.SetLevel(logger.LevelDebug)
		logger.Info("Verbose logging enabled")
	} else {
		logger.SetLevel(logger.LevelInfo)
	}

	state.Shared.Update(func(st *state.State) {
		st.ShowToken = opts.ShowToken
	})

	if err := paths.EnsurePaths(paths.Default); err != nil {
		return err
	}

	return token.SetupGitHubToken(ctx, state.Shared, paths.Default, token.SetupGitHubTokenOptions{Force: true}, http.DefaultClient)
}
