package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"internal/api"
	appErr "internal/errors"
	"internal/state"
)

type EmbeddingRequest struct {
	Input any    `json:"input"`
	Model string `json:"model"`
}

type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func CreateEmbeddings(ctx context.Context, s *state.State, client *http.Client, payload EmbeddingRequest) (*EmbeddingResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.CopilotBaseURL(s)+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for key, value := range api.CopilotHeaders(s, api.CopilotHeaderOptions{}) {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, appErr.NewHTTPError("Failed to create embeddings", resp)
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
