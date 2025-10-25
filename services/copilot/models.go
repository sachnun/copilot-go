package copilot

import (
	"context"
	"encoding/json"
	"net/http"

	"internal/api"
	appErr "internal/errors"
	"internal/state"
)

// ModelsResponse mirrors the JSON response from /models.
type ModelsResponse struct {
	Data   []Model `json:"data"`
	Object string  `json:"object"`
}

type Model struct {
	Capabilities       ModelCapabilities `json:"capabilities"`
	ID                 string            `json:"id"`
	ModelPickerEnabled bool              `json:"model_picker_enabled"`
	Name               string            `json:"name"`
	Object             string            `json:"object"`
	Preview            bool              `json:"preview"`
	Vendor             string            `json:"vendor"`
	Version            string            `json:"version"`
	Policy             *ModelPolicy      `json:"policy,omitempty"`
	SupportedEndpoints []string          `json:"supported_endpoints,omitempty"`
}

type ModelPolicy struct {
	State string `json:"state"`
	Terms string `json:"terms"`
}

type ModelCapabilities struct {
	Family    string        `json:"family"`
	Limits    ModelLimits   `json:"limits"`
	Object    string        `json:"object"`
	Supports  ModelSupports `json:"supports"`
	Tokenizer string        `json:"tokenizer"`
	Type      string        `json:"type"`
}

type ModelLimits struct {
	MaxContextWindowTokens *int `json:"max_context_window_tokens,omitempty"`
	MaxOutputTokens        *int `json:"max_output_tokens,omitempty"`
	MaxPromptTokens        *int `json:"max_prompt_tokens,omitempty"`
	MaxInputs              *int `json:"max_inputs,omitempty"`
}

type ModelSupports struct {
	ToolCalls         *bool `json:"tool_calls,omitempty"`
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
	Dimensions        *bool `json:"dimensions,omitempty"`
	Streaming         *bool `json:"streaming,omitempty"`
	StructuredOutputs *bool `json:"structured_outputs,omitempty"`
	Vision            *bool `json:"vision,omitempty"`
}

func GetModels(ctx context.Context, s *state.State, client *http.Client) (*ModelsResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api.CopilotBaseURL(s)+"/models", nil)
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
		return nil, appErr.NewHTTPError("Failed to get models", resp)
	}

	var models ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, err
	}

	return &models, nil
}
