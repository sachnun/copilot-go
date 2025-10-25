package messages

import (
	"encoding/json"
	"fmt"
)

// AnthropicMessagesPayload mirrors the Claude Messages API payload.
type AnthropicMessagesPayload struct {
	Model         string                   `json:"model"`
	Messages      []AnthropicMessage       `json:"messages"`
	MaxTokens     int                      `json:"max_tokens"`
	System        *AnthropicSystemPrompt   `json:"system,omitempty"`
	Metadata      *AnthropicMetadata       `json:"metadata,omitempty"`
	StopSequences []string                 `json:"stop_sequences,omitempty"`
	Stream        *bool                    `json:"stream,omitempty"`
	Temperature   *float64                 `json:"temperature,omitempty"`
	TopP          *float64                 `json:"top_p,omitempty"`
	TopK          *int                     `json:"top_k,omitempty"`
	Tools         []AnthropicTool          `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice     `json:"tool_choice,omitempty"`
	Thinking      *AnthropicThinkingConfig `json:"thinking,omitempty"`
	ServiceTier   *string                  `json:"service_tier,omitempty"`
}

type AnthropicSystemPrompt struct {
	String *string
	Blocks []AnthropicTextBlock
}

func (p *AnthropicSystemPrompt) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		p.String = &str
		return nil
	}
	var blocks []AnthropicTextBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		p.Blocks = blocks
		return nil
	}
	return fmt.Errorf("invalid system prompt payload: %s", string(data))
}

func (p AnthropicSystemPrompt) MarshalJSON() ([]byte, error) {
	switch {
	case p.String != nil:
		return json.Marshal(p.String)
	case len(p.Blocks) > 0:
		return json.Marshal(p.Blocks)
	default:
		return []byte("null"), nil
	}
}

type AnthropicMetadata struct {
	UserID *string `json:"user_id,omitempty"`
}

type AnthropicThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens *int   `json:"budget_tokens,omitempty"`
}

type AnthropicToolChoice struct {
	Type string  `json:"type"`
	Name *string `json:"name,omitempty"`
}

type AnthropicMessage struct {
	Role    string                  `json:"role"`
	Content AnthropicMessageContent `json:"content"`
}

type AnthropicMessageContent struct {
	String *string
	Blocks []AnthropicContentBlock
}

func (c *AnthropicMessageContent) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.String = &str
		return nil
	}
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		c.Blocks = blocks
		return nil
	}
	return fmt.Errorf("invalid message content: %s", string(data))
}

func (c AnthropicMessageContent) MarshalJSON() ([]byte, error) {
	switch {
	case c.String != nil:
		return json.Marshal(c.String)
	case len(c.Blocks) > 0:
		return json.Marshal(c.Blocks)
	default:
		return []byte("null"), nil
	}
}

type AnthropicContentBlock struct {
	Type      string
	Text      *string
	Source    *AnthropicImageSource
	ToolUseID *string
	Content   interface{}
	IsError   *bool
	ID        *string
	Name      *string
	Input     map[string]interface{}
	Thinking  *string
	Signature *string
}

func (b AnthropicContentBlock) AsText() (*AnthropicTextBlock, bool) {
	if b.Type != "text" || b.Text == nil {
		return nil, false
	}
	return &AnthropicTextBlock{Type: "text", Text: *b.Text}, true
}

func (b AnthropicContentBlock) AsThinking() (*AnthropicThinkingBlock, bool) {
	if b.Type != "thinking" || b.Thinking == nil {
		return nil, false
	}
	block := &AnthropicThinkingBlock{Type: "thinking", Thinking: *b.Thinking}
	if b.Signature != nil {
		block.Signature = *b.Signature
	}
	return block, true
}

func (b AnthropicContentBlock) AsImage() (*AnthropicImageBlock, bool) {
	if b.Type != "image" || b.Source == nil {
		return nil, false
	}
	return &AnthropicImageBlock{Type: "image", Source: *b.Source}, true
}

func (b AnthropicContentBlock) AsToolUse() (*AnthropicToolUseBlock, bool) {
	if b.Type != "tool_use" || b.ID == nil || b.Name == nil {
		return nil, false
	}
	input := b.Input
	if input == nil {
		input = map[string]interface{}{}
	}
	return &AnthropicToolUseBlock{
		Type:  "tool_use",
		ID:    *b.ID,
		Name:  *b.Name,
		Input: input,
	}, true
}

func (b AnthropicContentBlock) AsToolResult() (*AnthropicToolResultBlock, bool) {
	if b.Type != "tool_result" || b.ToolUseID == nil {
		return nil, false
	}
	return &AnthropicToolResultBlock{
		Type:      "tool_result",
		ToolUseID: *b.ToolUseID,
		Content:   b.Content,
		IsError:   b.IsError,
	}, true
}

func (b *AnthropicContentBlock) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if t, ok := raw["type"]; ok {
		if err := json.Unmarshal(t, &b.Type); err != nil {
			return err
		}
	}

	switch b.Type {
	case "text":
		var text string
		if err := json.Unmarshal(raw["text"], &text); err != nil {
			return err
		}
		b.Text = &text
	case "image":
		var source AnthropicImageSource
		if err := json.Unmarshal(raw["source"], &source); err != nil {
			return err
		}
		b.Source = &source
	case "tool_result":
		var toolUseID string
		if err := json.Unmarshal(raw["tool_use_id"], &toolUseID); err != nil {
			return err
		}
		b.ToolUseID = &toolUseID
		if contentRaw, ok := raw["content"]; ok {
			var content interface{}
			if err := json.Unmarshal(contentRaw, &content); err != nil {
				return err
			}
			b.Content = content
		}
		if isErr, ok := raw["is_error"]; ok {
			var flag bool
			if err := json.Unmarshal(isErr, &flag); err != nil {
				return err
			}
			b.IsError = &flag
		}
	case "tool_use":
		var id, name string
		if err := json.Unmarshal(raw["id"], &id); err != nil {
			return err
		}
		if err := json.Unmarshal(raw["name"], &name); err != nil {
			return err
		}
		b.ID = &id
		b.Name = &name
		var input map[string]interface{}
		if err := json.Unmarshal(raw["input"], &input); err != nil {
			return err
		}
		b.Input = input
	case "thinking":
		var thinking string
		if err := json.Unmarshal(raw["thinking"], &thinking); err != nil {
			return err
		}
		b.Thinking = &thinking
		if signatureRaw, ok := raw["signature"]; ok {
			var signature string
			if err := json.Unmarshal(signatureRaw, &signature); err == nil {
				b.Signature = &signature
			}
		}
	default:
		// Keep raw for unknown types to avoid data loss.
		var text string
		if err := json.Unmarshal(raw["text"], &text); err == nil {
			b.Text = &text
		}
		if sourceRaw, ok := raw["source"]; ok {
			var source AnthropicImageSource
			if err := json.Unmarshal(sourceRaw, &source); err == nil {
				b.Source = &source
			}
		}
	}

	return nil
}

func (b AnthropicContentBlock) MarshalJSON() ([]byte, error) {
	type alias struct {
		Type      string                 `json:"type"`
		Text      *string                `json:"text,omitempty"`
		Source    *AnthropicImageSource  `json:"source,omitempty"`
		ToolUseID *string                `json:"tool_use_id,omitempty"`
		Content   interface{}            `json:"content,omitempty"`
		IsError   *bool                  `json:"is_error,omitempty"`
		ID        *string                `json:"id,omitempty"`
		Name      *string                `json:"name,omitempty"`
		Input     map[string]interface{} `json:"input,omitempty"`
		Thinking  *string                `json:"thinking,omitempty"`
		Signature *string                `json:"signature,omitempty"`
	}
	return json.Marshal(alias{
		Type:      b.Type,
		Text:      b.Text,
		Source:    b.Source,
		ToolUseID: b.ToolUseID,
		Content:   b.Content,
		IsError:   b.IsError,
		ID:        b.ID,
		Name:      b.Name,
		Input:     b.Input,
		Thinking:  b.Thinking,
		Signature: b.Signature,
	})
}

type AnthropicTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicImageBlock struct {
	Type   string               `json:"type"`
	Source AnthropicImageSource `json:"source"`
}

type AnthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type AnthropicToolResultBlock struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
	IsError   *bool       `json:"is_error,omitempty"`
}

type AnthropicToolUseBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

type AnthropicThinkingBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type AnthropicResponse struct {
	ID           string                           `json:"id"`
	Type         string                           `json:"type"`
	Role         string                           `json:"role"`
	Content      []AnthropicAssistantContentBlock `json:"content"`
	Model        string                           `json:"model"`
	StopReason   *string                          `json:"stop_reason"`
	StopSequence *string                          `json:"stop_sequence"`
	Usage        AnthropicUsage                   `json:"usage"`
}

type AnthropicAssistantContentBlock struct {
	Type      string                 `json:"type"`
	Text      *string                `json:"text,omitempty"`
	ID        *string                `json:"id,omitempty"`
	Name      *string                `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Thinking  *string                `json:"thinking,omitempty"`
	Signature *string                `json:"signature,omitempty"`
}

type AnthropicUsage struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens *int    `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     *int    `json:"cache_read_input_tokens,omitempty"`
	ServiceTier              *string `json:"service_tier,omitempty"`
}

// Streaming types

type AnthropicStreamEventData interface{}

type AnthropicMessageStartEvent struct {
	Type    string `json:"type"`
	Message struct {
		ID           string         `json:"id"`
		Type         string         `json:"type"`
		Role         string         `json:"role"`
		Content      []interface{}  `json:"content"`
		Model        string         `json:"model"`
		StopReason   *string        `json:"stop_reason"`
		StopSequence *string        `json:"stop_sequence"`
		Usage        AnthropicUsage `json:"usage"`
	} `json:"message"`
}

type AnthropicContentBlockStartEvent struct {
	Type         string      `json:"type"`
	Index        int         `json:"index"`
	ContentBlock interface{} `json:"content_block"`
}

type AnthropicContentBlockDeltaEvent struct {
	Type  string      `json:"type"`
	Index int         `json:"index"`
	Delta interface{} `json:"delta"`
}

type AnthropicContentBlockStopEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type AnthropicMessageDeltaEvent struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason   *string `json:"stop_reason,omitempty"`
		StopSequence *string `json:"stop_sequence,omitempty"`
	} `json:"delta"`
	Usage *AnthropicUsage `json:"usage,omitempty"`
}

type AnthropicMessageStopEvent struct {
	Type string `json:"type"`
}

type AnthropicErrorEvent struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type AnthropicStreamState struct {
	MessageStartSent  bool
	ContentBlockIndex int
	ContentBlockOpen  bool
	ToolCalls         map[int]anthropicToolCallState
}

type anthropicToolCallState struct {
	ID                  string
	Name                string
	AnthropicBlockIndex int
}
