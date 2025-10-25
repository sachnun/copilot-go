package messages

import (
	"encoding/json"
	"strings"

	"internal/services/copilot"
)

// TranslateToOpenAI converts an Anthropic Messages payload into a Copilot chat payload.
func TranslateToOpenAI(payload AnthropicMessagesPayload) (copilot.ChatCompletionsPayload, error) {
	systemMessages, err := translateSystemPrompt(payload.System)
	if err != nil {
		return copilot.ChatCompletionsPayload{}, err
	}

	otherMessages, err := translateAnthropicMessages(payload.Messages)
	if err != nil {
		return copilot.ChatCompletionsPayload{}, err
	}

	chatMessages := append(systemMessages, otherMessages...)

	var maxTokens *int
	if payload.MaxTokens > 0 {
		value := payload.MaxTokens
		maxTokens = &value
	}

	var stop interface{}
	if len(payload.StopSequences) == 1 {
		stop = payload.StopSequences[0]
	} else if len(payload.StopSequences) > 1 {
		stop = payload.StopSequences
	}

	var stream *bool
	if payload.Stream != nil {
		stream = payload.Stream
	}

	var userID *string
	if payload.Metadata != nil && payload.Metadata.UserID != nil {
		userID = payload.Metadata.UserID
	}

	tools := translateAnthropicTools(payload.Tools)
	toolChoice := translateAnthropicToolChoice(payload.ToolChoice)

	return copilot.ChatCompletionsPayload{
		Model:       translateModelName(payload.Model),
		Messages:    chatMessages,
		MaxTokens:   maxTokens,
		Stop:        stop,
		Stream:      stream,
		Temperature: payload.Temperature,
		TopP:        payload.TopP,
		User:        userID,
		Tools:       tools,
		ToolChoice:  toolChoice,
	}, nil
}

func translateModelName(model string) string {
	if strings.HasPrefix(model, "claude-sonnet-4-") {
		return "claude-sonnet-4"
	}
	if strings.HasPrefix(model, "claude-opus-") {
		return "claude-opus-4"
	}
	return model
}

func translateSystemPrompt(system *AnthropicSystemPrompt) ([]copilot.Message, error) {
	if system == nil {
		return nil, nil
	}

	if system.String != nil {
		return []copilot.Message{{
			Role:    "system",
			Content: copilot.MessageContent{StringValue: system.String},
		}}, nil
	}

	if len(system.Blocks) > 0 {
		var texts []string
		for _, block := range system.Blocks {
			texts = append(texts, block.Text)
		}
		joined := joinWithDoubleNewline(texts)
		return []copilot.Message{
			{
				Role:    "system",
				Content: copilot.MessageContent{StringValue: &joined},
			},
		}, nil
	}

	return nil, nil
}

func translateAnthropicMessages(messages []AnthropicMessage) ([]copilot.Message, error) {
	var result []copilot.Message
	for _, message := range messages {
		switch message.Role {
		case "user":
			userMessages, err := handleUserMessage(message)
			if err != nil {
				return nil, err
			}
			result = append(result, userMessages...)
		case "assistant":
			assistantMessages, err := handleAssistantMessage(message)
			if err != nil {
				return nil, err
			}
			result = append(result, assistantMessages...)
		default:
			// Unsupported roles are ignored gracefully.
		}
	}
	return result, nil
}

func handleUserMessage(message AnthropicMessage) ([]copilot.Message, error) {
	if message.Content.String != nil {
		return []copilot.Message{
			{
				Role:    "user",
				Content: copilot.MessageContent{StringValue: message.Content.String},
			},
		}, nil
	}

	var result []copilot.Message
	var pending []AnthropicContentBlock

	for _, block := range message.Content.Blocks {
		if toolResult, ok := block.AsToolResult(); ok {
			if len(pending) > 0 {
				content, err := mapContentBlocks(pending)
				if err != nil {
					return nil, err
				}
				result = append(result, copilot.Message{
					Role:    "user",
					Content: content,
				})
				pending = nil
			}
			msgContent, err := mapGenericContent(toolResult.Content)
			if err != nil {
				return nil, err
			}
			toolCallID := toolResult.ToolUseID
			result = append(result, copilot.Message{
				Role:       "tool",
				ToolCallID: &toolCallID,
				Content:    msgContent,
			})
			continue
		}
		pending = append(pending, block)
	}

	if len(pending) > 0 {
		content, err := mapContentBlocks(pending)
		if err != nil {
			return nil, err
		}
		result = append(result, copilot.Message{
			Role:    "user",
			Content: content,
		})
	}

	if len(result) == 0 {
		result = append(result, copilot.Message{
			Role:    "user",
			Content: copilot.MessageContent{},
		})
	}

	return result, nil
}

func handleAssistantMessage(message AnthropicMessage) ([]copilot.Message, error) {
	if message.Content.String != nil {
		return []copilot.Message{
			{
				Role:    "assistant",
				Content: copilot.MessageContent{StringValue: message.Content.String},
			},
		}, nil
	}

	toolUseBlocks := make([]AnthropicToolUseBlock, 0)
	textParts := make([]string, 0)
	hasImage := false

	for _, block := range message.Content.Blocks {
		if toolUse, ok := block.AsToolUse(); ok {
			toolUseBlocks = append(toolUseBlocks, *toolUse)
			continue
		}
		if text, ok := block.AsText(); ok {
			textParts = append(textParts, text.Text)
			continue
		}
		if thinking, ok := block.AsThinking(); ok {
			textParts = append(textParts, thinking.Thinking)
			continue
		}
		if _, ok := block.AsImage(); ok {
			hasImage = true
		}
	}

	var messages []copilot.Message
	if len(toolUseBlocks) > 0 {
		content := strings.Join(textParts, "\n\n")
		var msgContent copilot.MessageContent
		if content != "" {
			msgContent = copilot.MessageContent{StringValue: &content}
		}
		msg := copilot.Message{
			Role:    "assistant",
			Content: msgContent,
		}
		for _, toolBlock := range toolUseBlocks {
			arguments := "{}"
			if toolBlock.Input != nil {
				inputMap, err := ensureMap(toolBlock.Input)
				if err != nil {
					return nil, err
				}
				bytes, err := json.Marshal(inputMap)
				if err != nil {
					return nil, err
				}
				arguments = string(bytes)
			}

			msg.ToolCalls = append(msg.ToolCalls, copilot.ToolCall{
				ID:   toolBlock.ID,
				Type: "function",
				Function: copilot.ToolCallFunction{
					Name:      toolBlock.Name,
					Arguments: arguments,
				},
			})
		}
		messages = append(messages, msg)
	} else if hasImage {
		content, err := mapContentBlocks(message.Content.Blocks)
		if err != nil {
			return nil, err
		}
		messages = append(messages, copilot.Message{
			Role:    "assistant",
			Content: content,
		})
	} else {
		joined := strings.Join(textParts, "\n\n")
		messages = append(messages, copilot.Message{
			Role:    "assistant",
			Content: copilot.MessageContent{StringValue: &joined},
		})
	}

	return messages, nil
}

func mapContentBlocks(blocks []AnthropicContentBlock) (copilot.MessageContent, error) {
	hasImage := false
	for _, block := range blocks {
		if block.Type == "image" {
			hasImage = true
			break
		}
	}

	if !hasImage {
		var texts []string
		for _, block := range blocks {
			if text, ok := block.AsText(); ok {
				texts = append(texts, text.Text)
			} else if thinking, ok := block.AsThinking(); ok {
				texts = append(texts, thinking.Thinking)
			}
		}
		joined := joinWithDoubleNewline(texts)
		if joined == "" {
			return copilot.MessageContent{}, nil
		}
		return copilot.MessageContent{StringValue: &joined}, nil
	}

	var parts []copilot.ContentPart
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != nil {
				text := *block.Text
				parts = append(parts, copilot.ContentPart{
					Type: "text",
					Text: &text,
				})
			}
		case "thinking":
			if block.Thinking != nil {
				text := *block.Thinking
				parts = append(parts, copilot.ContentPart{
					Type: "text",
					Text: &text,
				})
			}
		case "image":
			if block.Source != nil {
				url := "data:" + block.Source.MediaType + ";base64," + block.Source.Data
				parts = append(parts, copilot.ContentPart{
					Type: "image_url",
					ImageURL: &copilot.ContentImage{
						URL:    url,
						Detail: stringPtr("auto"),
					},
				})
			}
		}
	}

	return copilot.MessageContent{Parts: parts}, nil
}

func mapGenericContent(value interface{}) (copilot.MessageContent, error) {
	switch content := value.(type) {
	case string:
		return copilot.MessageContent{StringValue: &content}, nil
	case []AnthropicContentBlock:
		return mapContentBlocks(content)
	case []interface{}:
		blocks, err := ensureBlocksSlice(content)
		if err != nil {
			return copilot.MessageContent{}, err
		}
		return mapContentBlocks(blocks)
	default:
		if content == nil {
			return copilot.MessageContent{}, nil
		}
		blocks, err := ensureBlocksSlice(content)
		if err != nil {
			return copilot.MessageContent{}, err
		}
		return mapContentBlocks(blocks)
	}
}

func translateAnthropicTools(tools []AnthropicTool) []copilot.Tool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]copilot.Tool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, copilot.Tool{
			Type: "function",
			Function: copilot.Function{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return result
}

func translateAnthropicToolChoice(choice *AnthropicToolChoice) *copilot.ToolChoice {
	if choice == nil {
		return nil
	}

	switch choice.Type {
	case "auto":
		value := "auto"
		return &copilot.ToolChoice{StringValue: &value}
	case "any":
		value := "required"
		return &copilot.ToolChoice{StringValue: &value}
	case "tool":
		if choice.Name != nil {
			return &copilot.ToolChoice{
				ObjectValue: &copilot.ToolChoiceValue{
					Type: "function",
					Function: copilot.ToolChoiceFunctionValue{
						Name: *choice.Name,
					},
				},
			}
		}
	case "none":
		value := "none"
		return &copilot.ToolChoice{StringValue: &value}
	}

	return nil
}
