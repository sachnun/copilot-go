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

type GitHubUser struct {
	Login string `json:"login"`
}

func GetUser(ctx context.Context, s *state.State, client *http.Client) (*GitHubUser, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/user", api.GitHubAPIBaseURL), nil)
	if err != nil {
		return nil, err
	}

	for key, value := range api.StandardHeaders() {
		req.Header.Set(key, value)
	}

	token := ""
	s.Read(func(st *state.State) {
		token = st.GitHubToken
	})
	if token == "" {
		return nil, fmt.Errorf("github token not set")
	}
	req.Header.Set("authorization", "token "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.NewHTTPError("Failed to get GitHub user", resp)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}
