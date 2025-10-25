package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"internal/api"
	"internal/errors"
	"internal/state"
)

type CopilotTokenResponse struct {
	ExpiresAt int64  `json:"expires_at"`
	RefreshIn int64  `json:"refresh_in"`
	Token     string `json:"token"`
}

func GetCopilotToken(ctx context.Context, s *state.State, client *http.Client) (*CopilotTokenResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/copilot_internal/v2/token", api.GitHubAPIBaseURL), nil)
	if err != nil {
		return nil, err
	}

	for key, value := range api.GitHubHeaders(s) {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.NewHTTPError("Failed to get Copilot token", resp)
	}

	var token CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}
