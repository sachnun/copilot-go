package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"internal/api"
	appErr "internal/errors"
	"internal/logger"
	"internal/state"
)

type ResponsesPayload struct {
	Model  string          `json:"model"`
	Input  json.RawMessage `json:"input,omitempty"`
	Stream *bool           `json:"stream,omitempty"`
}

type ResponsesResult map[string]any
type ResponsesStream <-chan SSEMessage

type ResponsesRequestOptions struct {
	Vision    bool
	Initiator string
	Stream    bool
}

func (p ResponsesPayload) StreamEnabled() bool {
	return p.Stream != nil && *p.Stream
}

func (p ResponsesPayload) InputItems() []any {
	if len(p.Input) == 0 {
		return nil
	}

	var array []any
	if err := json.Unmarshal(p.Input, &array); err == nil {
		return array
	}
	return nil
}

func HasVisionInput(payload ResponsesPayload) bool {
	for _, item := range payload.InputItems() {
		if containsVisionContent(item) {
			return true
		}
	}
	return false
}

func containsVisionContent(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case []any:
		for _, item := range v {
			if containsVisionContent(item) {
				return true
			}
		}
	case map[string]any:
		if typeValue, ok := v["type"].(string); ok && strings.ToLower(typeValue) == "input_image" {
			return true
		}
		if content, ok := v["content"].([]any); ok {
			for _, item := range content {
				if containsVisionContent(item) {
					return true
				}
			}
		}
	}
	return false
}
func sanitizeResponsesPayload(raw []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	delete(payload, "store")
	return json.Marshal(payload)
}

func CreateResponses(ctx context.Context, s *state.State, rawPayload []byte, opts ResponsesRequestOptions, client *http.Client, streamer SSEReader) (interface{}, error) {
	if client == nil {
		client = http.DefaultClient
	}

	body, err := sanitizeResponsesPayload(rawPayload)
	if err != nil {
		return nil, err
	}

	logger.Debug("Calling Copilot responses model stream=%v", opts.Stream)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.CopilotBaseURL(s)+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	headers := api.CopilotHeaders(s, api.CopilotHeaderOptions{
		Vision:    opts.Vision,
		Initiator: opts.Initiator,
	})
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		logger.Error("Copilot responses failed with status %d", resp.StatusCode)
		return nil, appErr.NewHTTPError("Failed to create responses", resp)
	}

	if opts.Stream {
		logger.Debug("Copilot responses streaming response acknowledged")
		stream, err := streamer.ReadSSE(ctx, resp)
		if err != nil {
			return nil, err
		}
		return ResponsesStream(stream), nil
	}

	defer resp.Body.Close()
	var result ResponsesResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	logger.Debug("Copilot responses result received: id=%v", result["id"])
	return result, nil
}
