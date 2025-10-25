package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"internal/api"
)

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

func PollAccessToken(ctx context.Context, device *DeviceCodeResponse, client *http.Client) (string, error) {
	if device == nil {
		return "", errors.New("device code response is nil")
	}

	if client == nil {
		client = http.DefaultClient
	}

	sleepDuration := time.Duration(device.Interval+1) * time.Second

	for {
		body := `{"client_id":"` + api.GitHubClientID + `","device_code":"` + device.DeviceCode + `","grant_type":"urn:ietf:params:oauth:grant-type:device_code"}`
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.GitHubBaseURL+"/login/oauth/access_token", strings.NewReader(body))
		if err != nil {
			return "", err
		}
		for key, value := range api.StandardHeaders() {
			req.Header.Set(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(sleepDuration):
				continue
			}
		}

		var decoded AccessTokenResponse
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.NewDecoder(resp.Body).Decode(&decoded); err == nil && decoded.AccessToken != "" {
				resp.Body.Close()
				return decoded.AccessToken, nil
			}
		}
		resp.Body.Close()

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(sleepDuration):
		}
	}
}
