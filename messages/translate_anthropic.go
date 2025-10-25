package messages

import (
	"encoding/json"
	"strings"

	"internal/services/copilot"
)

func TranslateToAnthropic(response copilot.ChatCompletionResponse) (AnthropicResponse, error) {
	var textBlocks []AnthropicAssistantContentBlock
	var toolBlocks []AnthropicAssistantContentBlock

	var stopReason *string
	if len(response.Choices) > 0 {
		stopReason = mapFinishReason(response.Choices[0].FinishReason)
	}

	for _, choice := range response.Choices {
		textBlocks = append(textBlocks, getAnthropicTextBlocks(choice.Message.Content)...)
		toolBlocks = append(toolBlocks, getAnthropicToolUseBlocks(choice.Message.ToolCalls)...)

		if stopReason == nil {
			stopReason = mapFinishReason(choice.FinishReason)
		}
		if choice.FinishReason == "tool_calls" {
			mapped := "tool_use"
			stopReason = &mapped
		}
	}

	usage := AnthropicUsage{
		InputTokens:  usagePromptTokens(response),
		OutputTokens: usageCompletionTokens(response),
	}
	if cached := usageCachedPromptTokens(response); cached != nil {
		usage.CacheReadInputTokens = cached
	}

	content := append(textBlocks, toolBlocks...)
	return AnthropicResponse{
		ID:           response.ID,
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		Model:        response.Model,
		StopReason:   stopReason,
		StopSequence: nil,
		Usage:        usage,
	}, nil
}

func getAnthropicTextBlocks(content copilot.MessageContent) []AnthropicAssistantContentBlock {
	if content.StringValue != nil {
		text := *content.StringValue
		return []AnthropicAssistantContentBlock{{Type: "text", Text: &text}}
	}

	if len(content.Parts) == 0 {
		return nil
	}

	var texts []string
	for _, part := range content.Parts {
		if part.Type == "text" && part.Text != nil {
			texts = append(texts, *part.Text)
		}
	}
	if len(texts) == 0 {
		return nil
	}
	joined := strings.Join(texts, "\n\n")
	return []AnthropicAssistantContentBlock{{Type: "text", Text: &joined}}
}

func getAnthropicToolUseBlocks(toolCalls []copilot.ToolCall) []AnthropicAssistantContentBlock {
	if len(toolCalls) == 0 {
		return nil
	}
	blocks := make([]AnthropicAssistantContentBlock, 0, len(toolCalls))
	for _, call := range toolCalls {
		input := make(map[string]interface{})
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &input); err != nil {
				input = map[string]interface{}{"raw_arguments": call.Function.Arguments}
			}
		}
		name := call.Function.Name
		id := call.ID
		blocks = append(blocks, AnthropicAssistantContentBlock{
			Type:  "tool_use",
			ID:    &id,
			Name:  &name,
			Input: input,
		})
	}
	return blocks
}

func mapFinishReason(reason string) *string {
	if reason == "" {
		return nil
	}
	return mapOpenAIStopReasonToAnthropic(&reason)
}

func usagePromptTokens(resp copilot.ChatCompletionResponse) int {
	if resp.Usage == nil {
		return 0
	}
	tokens := resp.Usage.PromptTokens
	if resp.Usage.PromptTokensDetails != nil {
		tokens -= resp.Usage.PromptTokensDetails.CachedTokens
	}
	return tokens
}

func usageCompletionTokens(resp copilot.ChatCompletionResponse) int {
	if resp.Usage == nil {
		return 0
	}
	return resp.Usage.CompletionTokens
}

func usageCachedPromptTokens(resp copilot.ChatCompletionResponse) *int {
	if resp.Usage == nil || resp.Usage.PromptTokensDetails == nil {
		return nil
	}
	value := resp.Usage.PromptTokensDetails.CachedTokens
	return &value
}
