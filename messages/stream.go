package messages

import (
	"encoding/json"

	"internal/services/copilot"
)

func NewStreamState() AnthropicStreamState {
	return AnthropicStreamState{
		MessageStartSent:  false,
		ContentBlockIndex: 0,
		ContentBlockOpen:  false,
		ToolCalls:         make(map[int]anthropicToolCallState),
	}
}

// TranslateChunkToAnthropicEvents converts a Copilot stream chunk to Anthropic stream events.
type SSEEvent struct {
	Type string
	Data json.RawMessage
}

func TranslateChunkToAnthropicEvents(chunk copilot.ChatCompletionChunk, state *AnthropicStreamState) ([]SSEEvent, error) {
	events := make([]SSEEvent, 0)

	if len(chunk.Choices) == 0 {
		return events, nil
	}

	choice := chunk.Choices[0]
	delta := choice.Delta

	if !state.MessageStartSent {
		event := AnthropicMessageStartEvent{
			Type: "message_start",
		}
		event.Message.ID = chunk.ID
		event.Message.Type = "message"
		event.Message.Role = "assistant"
		event.Message.Content = []interface{}{}
		event.Message.Model = chunk.Model
		event.Message.StopReason = nil
		event.Message.StopSequence = nil
		usage := AnthropicUsage{
			InputTokens:  chunkUsagePromptTokens(chunk),
			OutputTokens: 0,
		}
		if cached := chunkUsageCachedPromptTokens(chunk); cached != nil {
			value := *cached
			usage.CacheReadInputTokens = &value
		}
		event.Message.Usage = usage

		data, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		events = append(events, SSEEvent{Type: event.Type, Data: data})
		state.MessageStartSent = true
	}

	if delta.Content != nil {
		if state.ContentBlockOpen && isToolBlockOpen(state) {
			stopEvent := AnthropicContentBlockStopEvent{
				Type:  "content_block_stop",
				Index: state.ContentBlockIndex,
			}
			if data, err := json.Marshal(stopEvent); err == nil {
				events = append(events, SSEEvent{Type: stopEvent.Type, Data: data})
			} else {
				return nil, err
			}
			state.ContentBlockIndex++
			state.ContentBlockOpen = false
		}

		if !state.ContentBlockOpen {
			startEvent := AnthropicContentBlockStartEvent{
				Type:  "content_block_start",
				Index: state.ContentBlockIndex,
				ContentBlock: map[string]string{
					"type": "text",
					"text": "",
				},
			}
			if data, err := json.Marshal(startEvent); err == nil {
				events = append(events, SSEEvent{Type: startEvent.Type, Data: data})
			} else {
				return nil, err
			}
			state.ContentBlockOpen = true
		}

		deltaEvent := AnthropicContentBlockDeltaEvent{
			Type:  "content_block_delta",
			Index: state.ContentBlockIndex,
			Delta: map[string]string{
				"type": "text_delta",
				"text": *delta.Content,
			},
		}
		if data, err := json.Marshal(deltaEvent); err == nil {
			events = append(events, SSEEvent{Type: deltaEvent.Type, Data: data})
		} else {
			return nil, err
		}
	}

	if len(delta.ToolCalls) > 0 {
		for _, toolCall := range delta.ToolCalls {
			if toolCall.ID != "" && toolCall.Function.Name != "" {
				if state.ContentBlockOpen {
					stopEvent := AnthropicContentBlockStopEvent{
						Type:  "content_block_stop",
						Index: state.ContentBlockIndex,
					}
					if data, err := json.Marshal(stopEvent); err == nil {
						events = append(events, SSEEvent{Type: stopEvent.Type, Data: data})
					} else {
						return nil, err
					}
					state.ContentBlockIndex++
					state.ContentBlockOpen = false
				}

				state.ToolCalls[toolCall.Index] = anthropicToolCallState{
					ID:                  toolCall.ID,
					Name:                toolCall.Function.Name,
					AnthropicBlockIndex: state.ContentBlockIndex,
				}

				startEvent := AnthropicContentBlockStartEvent{
					Type:  "content_block_start",
					Index: state.ContentBlockIndex,
					ContentBlock: map[string]interface{}{
						"type":  "tool_use",
						"id":    toolCall.ID,
						"name":  toolCall.Function.Name,
						"input": map[string]interface{}{},
					},
				}
				if data, err := json.Marshal(startEvent); err == nil {
					events = append(events, SSEEvent{Type: startEvent.Type, Data: data})
				} else {
					return nil, err
				}
				state.ContentBlockOpen = true
			}

			if toolCall.Function.Arguments != "" {
				if info, ok := state.ToolCalls[toolCall.Index]; ok {
					deltaEvent := AnthropicContentBlockDeltaEvent{
						Type:  "content_block_delta",
						Index: info.AnthropicBlockIndex,
						Delta: map[string]string{
							"type":         "input_json_delta",
							"partial_json": toolCall.Function.Arguments,
						},
					}
					if data, err := json.Marshal(deltaEvent); err == nil {
						events = append(events, SSEEvent{Type: deltaEvent.Type, Data: data})
					} else {
						return nil, err
					}
				}
			}
		}
	}

	if choice.FinishReason != nil && *choice.FinishReason != "" {
		if state.ContentBlockOpen {
			stopEvent := AnthropicContentBlockStopEvent{
				Type:  "content_block_stop",
				Index: state.ContentBlockIndex,
			}
			if data, err := json.Marshal(stopEvent); err == nil {
				events = append(events, SSEEvent{Type: stopEvent.Type, Data: data})
			} else {
				return nil, err
			}
			state.ContentBlockOpen = false
		}

		reason := *choice.FinishReason
		mapped := mapOpenAIStopReasonToAnthropic(&reason)
		messageDelta := AnthropicMessageDeltaEvent{
			Type: "message_delta",
		}
		if mapped != nil {
			messageDelta.Delta.StopReason = mapped
		}
		messageDelta.Delta.StopSequence = nil
		usage := AnthropicUsage{
			InputTokens:  chunkUsagePromptTokens(chunk),
			OutputTokens: chunkUsageCompletionTokens(chunk),
		}
		if cached := chunkUsageCachedPromptTokens(chunk); cached != nil {
			value := *cached
			usage.CacheReadInputTokens = &value
		}
		messageDelta.Usage = &usage

		if data, err := json.Marshal(messageDelta); err == nil {
			events = append(events, SSEEvent{Type: messageDelta.Type, Data: data})
		} else {
			return nil, err
		}

		stopEvent := AnthropicMessageStopEvent{Type: "message_stop"}
		if data, err := json.Marshal(stopEvent); err == nil {
			events = append(events, SSEEvent{Type: stopEvent.Type, Data: data})
		} else {
			return nil, err
		}
	}

	return events, nil
}

func TranslateStreamError() SSEEvent {
	event := AnthropicErrorEvent{
		Type: "error",
	}
	event.Error.Type = "api_error"
	event.Error.Message = "An unexpected error occurred during streaming."
	data, _ := json.Marshal(event)
	return SSEEvent{Type: event.Type, Data: data}
}

func isToolBlockOpen(state *AnthropicStreamState) bool {
	if !state.ContentBlockOpen {
		return false
	}
	for _, tc := range state.ToolCalls {
		if tc.AnthropicBlockIndex == state.ContentBlockIndex {
			return true
		}
	}
	return false
}

func chunkUsagePromptTokens(chunk copilot.ChatCompletionChunk) int {
	if chunk.Usage == nil {
		return 0
	}
	tokens := chunk.Usage.PromptTokens
	if chunk.Usage.PromptTokensDetails != nil {
		tokens -= chunk.Usage.PromptTokensDetails.CachedTokens
	}
	return tokens
}

func chunkUsageCachedPromptTokens(chunk copilot.ChatCompletionChunk) *int {
	if chunk.Usage == nil || chunk.Usage.PromptTokensDetails == nil {
		return nil
	}
	value := chunk.Usage.PromptTokensDetails.CachedTokens
	return &value
}

func chunkUsageCompletionTokens(chunk copilot.ChatCompletionChunk) int {
	if chunk.Usage == nil {
		return 0
	}
	return chunk.Usage.CompletionTokens
}
