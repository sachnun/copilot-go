package token

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"internal/logger"
	"internal/paths"
	"internal/services/github"
	"internal/state"
)

type SetupGitHubTokenOptions struct {
	Force bool
}

func SetupGitHubToken(ctx context.Context, s *state.State, p paths.Paths, opts SetupGitHubTokenOptions, client *http.Client) error {
	if client == nil {
		client = http.DefaultClient
	}

	existing, err := os.ReadFile(p.GitHubToken)
	if err == nil {
		token := strings.TrimSpace(string(existing))
		if token != "" && !opts.Force {
			s.Update(func(st *state.State) {
				st.GitHubToken = token
			})
			showToken := false
			s.Read(func(st *state.State) { showToken = st.ShowToken })
			if showToken {
				logger.Info("GitHub token: %s", token)
			}

			return logUser(ctx, s, client)
		}
	}

	logger.Info("Not logged in, getting new access token")
	device, err := github.GetDeviceCode(ctx, client)
	if err != nil {
		return err
	}
	logger.Debug("Device code response: %+v", device)
	logger.Info("Please enter the code %q in %s", device.UserCode, device.VerificationURI)

	token, err := github.PollAccessToken(ctx, device, client)
	if err != nil {
		return err
	}

	if err := os.WriteFile(p.GitHubToken, []byte(token), 0o600); err != nil {
		return err
	}

	s.Update(func(st *state.State) {
		st.GitHubToken = token
	})

	showToken := false
	s.Read(func(st *state.State) { showToken = st.ShowToken })
	if showToken {
		logger.Info("GitHub token: %s", token)
	}

	logger.Info("GitHub token written to %s", p.GitHubToken)
	return logUser(ctx, s, client)
}

func logUser(ctx context.Context, s *state.State, client *http.Client) error {
	user, err := github.GetUser(ctx, s, client)
	if err != nil {
		return err
	}
	logger.Info("Logged in as %s", user.Login)
	return nil
}

func SetupCopilotToken(ctx context.Context, s *state.State, client *http.Client) (context.CancelFunc, error) {
	if client == nil {
		client = http.DefaultClient
	}

	tokenResp, err := github.GetCopilotToken(ctx, s, client)
	if err != nil {
		return nil, err
	}

	logger.Debug("GitHub Copilot Token fetched successfully!")
	updateCopilotToken(s, tokenResp.Token)

	refreshAfter := time.Duration(tokenResp.RefreshIn-60) * time.Second
	if refreshAfter <= 0 {
		refreshAfter = 5 * time.Minute
	}

	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(refreshAfter)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				logger.Debug("Refreshing Copilot token")
				resp, err := github.GetCopilotToken(context.Background(), s, client)
				if err != nil {
					logger.Error("Failed to refresh Copilot token: %v", err)
					continue
				}
				updateCopilotToken(s, resp.Token)
				logger.Debug("Copilot token refreshed")
			}
		}
	}()

	return cancel, nil
}

func updateCopilotToken(s *state.State, token string) {
	s.Update(func(st *state.State) {
		st.CopilotToken = token
	})

	showToken := false
	s.Read(func(st *state.State) { showToken = st.ShowToken })
	if showToken {
		logger.Info("Copilot token: %s", token)
	}
}
