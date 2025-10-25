package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"internal/api"
	"internal/errors"
	"internal/logger"
	"internal/state"
)

// ChatCompletionsPayload mirrors the TypeScript payload definition.
type ChatCompletionsPayload struct {
	Messages         []Message          `json:"messages"`
	Model            string             `json:"model"`
	Temperature      *float64           `json:"temperature,omitempty"`
	TopP             *float64           `json:"top_p,omitempty"`
	MaxTokens        *int               `json:"max_tokens,omitempty"`
	Stop             interface{}        `json:"stop,omitempty"` // string or []string or null
	N                *int               `json:"n,omitempty"`
	Stream           *bool              `json:"stream,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
	Logprobs         *bool              `json:"logprobs,omitempty"`
	ResponseFormat   *ResponseFormat    `json:"response_format,omitempty"`
	Seed             *int               `json:"seed,omitempty"`
	Tools            []Tool             `json:"tools,omitempty"`
	ToolChoice       *ToolChoice        `json:"tool_choice,omitempty"`
	User             *string            `json:"user,omitempty"`
}

// ResponseFormat mirrors { type: "json_object" } | null
type ResponseFormat struct {
	Type string `json:"type"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string      `json:"name"`
	Description *string     `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters"`
}

// ToolChoice can be a string enum or an object.
type ToolChoice struct {
	StringValue *string          `json:"-"`
	ObjectValue *ToolChoiceValue `json:"-"`
}

type ToolChoiceValue struct {
	Type     string                  `json:"type"`
	Function ToolChoiceFunctionValue `json:"function"`
}

type ToolChoiceFunctionValue struct {
	Name string `json:"name"`
}

// MarshalJSON implements custom JSON marshaling to keep type parity.
func (t ToolChoice) MarshalJSON() ([]byte, error) {
	switch {
	case t.StringValue != nil:
		return json.Marshal(t.StringValue)
	case t.ObjectValue != nil:
		return json.Marshal(t.ObjectValue)
	default:
		return []byte("null"), nil
	}
}

// UnmarshalJSON handles union representation.
func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		t.StringValue = &str
		return nil
	}

	var obj ToolChoiceValue
	if err := json.Unmarshal(data, &obj); err == nil {
		t.ObjectValue = &obj
		return nil
	}

	if string(data) == "null" {
		return nil
	}

	return fmt.Errorf("unsupported tool_choice payload: %s", string(data))
}

type Message struct {
	Role       string         `json:"role"`
	Content    MessageContent `json:"content"`
	Name       *string        `json:"name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID *string        `json:"tool_call_id,omitempty"`
}

// MessageContent can be string, []ContentPart or null.
type MessageContent struct {
	StringValue *string       `json:"-"`
	Parts       []ContentPart `json:"-"`
}

type ContentPart struct {
	Type     string        `json:"type"`
	Text     *string       `json:"text,omitempty"`
	ImageURL *ContentImage `json:"image_url,omitempty"`
}

type ContentImage struct {
	URL    string  `json:"url"`
	Detail *string `json:"detail,omitempty"`
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
	switch {
	case c.StringValue != nil:
		return json.Marshal(c.StringValue)
	case len(c.Parts) > 0:
		return json.Marshal(c.Parts)
	default:
		return []byte("null"), nil
	}
}

func (c *MessageContent) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.StringValue = &str
		return nil
	}

	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		c.Parts = parts
		return nil
	}

	return fmt.Errorf("unsupported message content: %s", string(data))
}

type ToolCall struct {
	Index    int              `json:"index,omitempty"`
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatCompletionResponse struct {
	ID                string               `json:"id"`
	Object            string               `json:"object"`
	Created           int64                `json:"created"`
	Model             string               `json:"model"`
	Choices           []ChoiceNonStreaming `json:"choices"`
	SystemFingerprint *string              `json:"system_fingerprint,omitempty"`
	Usage             *ChatUsage           `json:"usage,omitempty"`
}

type ChatUsage struct {
	PromptTokens        int                  `json:"prompt_tokens"`
	CompletionTokens    int                  `json:"completion_tokens"`
	TotalTokens         int                  `json:"total_tokens"`
	PromptTokensDetails *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type Choice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
	Logprobs     any     `json:"logprobs"`
}

type ChoiceNonStreaming struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	Logprobs     any             `json:"logprobs"`
	FinishReason string          `json:"finish_reason"`
}

type ResponseMessage struct {
	Role      string         `json:"role"`
	Content   MessageContent `json:"content"`
	ToolCalls []ToolCall     `json:"tool_calls,omitempty"`
}

type Delta struct {
	Content   *string    `json:"content,omitempty"`
	Role      *string    `json:"role,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ChatCompletionChunk struct {
	ID                string        `json:"id"`
	Object            string        `json:"object"`
	Created           int64         `json:"created"`
	Model             string        `json:"model"`
	Choices           []Choice      `json:"choices"`
	SystemFingerprint *string       `json:"system_fingerprint,omitempty"`
	Usage             *UsageDetails `json:"usage,omitempty"`
}

type UsageDetails struct {
	PromptTokens            int                      `json:"prompt_tokens"`
	CompletionTokens        int                      `json:"completion_tokens"`
	TotalTokens             int                      `json:"total_tokens"`
	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

type CompletionTokensDetails struct {
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
}

type SSEReader interface {
	ReadSSE(ctx context.Context, resp *http.Response) (<-chan SSEMessage, error)
}

type SSEMessage struct {
	Event string
	Data  string
}

// CreateChatCompletions proxies the request to the Copilot endpoint.
func CreateChatCompletions(ctx context.Context, s *state.State, payload ChatCompletionsPayload, httpClient *http.Client, streamer SSEReader) (interface{}, error) {
	url := fmt.Sprintf("%s/chat/completions", api.CopilotBaseURL(s))
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	logger.Debug("Calling Copilot chat completions model=%s stream=%v", payload.Model, payload.Stream != nil && *payload.Stream)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}

	for key, value := range api.CopilotHeaders(s, api.CopilotHeaderOptions{
		Vision:    payload.ContainsVision(),
		Initiator: ResolveChatInitiator(payload.Model, payload.Messages),
	}) {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		logger.Error("Copilot chat completions failed with status %d", resp.StatusCode)
		return nil, errors.NewHTTPError("Failed to create chat completions", resp)
	}

	stream := payload.Stream != nil && *payload.Stream
	if stream {
		logger.Debug("Copilot chat completion streaming response acknowledged")
		return streamer.ReadSSE(ctx, resp)
	}

	defer resp.Body.Close()
	var parsed ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	logger.Debug("Copilot chat completion response received: id=%s", parsed.ID)
	return parsed, nil
}

// ResolveChatInitiator follows the TypeScript logic.
func ResolveChatInitiator(model string, messages []Message) string {
	hasAgent := false
	for _, msg := range messages {
		role := strings.ToLower(msg.Role)
		if role == "assistant" || role == "tool" {
			hasAgent = true
			break
		}
	}

	if hasAgent || strings.HasPrefix(model, "claude-") || model == "gpt-5-codex" {
		return "agent"
	}
	return "user"
}

// ContainsVision determines if payload uses vision content.
func (p ChatCompletionsPayload) ContainsVision() bool {
	for _, msg := range p.Messages {
		for _, part := range msg.Content.Parts {
			if part.Type == "image_url" {
				return true
			}
		}
	}
	return false
}
