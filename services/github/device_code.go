package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"internal/api"
	"internal/errors"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int64  `json:"expires_in"`
	Interval        int64  `json:"interval"`
}

func GetDeviceCode(ctx context.Context, client *http.Client) (*DeviceCodeResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}

	body := `{"client_id":"` + api.GitHubClientID + `","scope":"` + api.GitHubAppScopes + `"}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.GitHubBaseURL+"/login/device/code", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range api.StandardHeaders() {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.NewHTTPError("Failed to get device code", resp)
	}

	var decoded DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	return &decoded, nil
}
