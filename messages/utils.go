package messages

import (
	"encoding/json"
	"fmt"
)

func mapOpenAIStopReasonToAnthropic(reason *string) *string {
	if reason == nil {
		return nil
	}
	mapping := map[string]string{
		"stop":           "end_turn",
		"length":         "max_tokens",
		"tool_calls":     "tool_use",
		"content_filter": "end_turn",
	}
	if mapped, ok := mapping[*reason]; ok {
		return &mapped
	}
	return nil
}

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}

func toJSONRaw(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func ensureMap(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return map[string]interface{}{}, nil
	}
	if m, ok := value.(map[string]interface{}); ok {
		return m, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var target map[string]interface{}
	if err := json.Unmarshal(data, &target); err != nil {
		return nil, err
	}
	return target, nil
}

func ensureBlocksSlice(value interface{}) ([]AnthropicContentBlock, error) {
	if value == nil {
		return nil, nil
	}
	switch val := value.(type) {
	case []AnthropicContentBlock:
		return val, nil
	case []interface{}:
		blocks := make([]AnthropicContentBlock, 0, len(val))
		for _, item := range val {
			data, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			var block AnthropicContentBlock
			if err := json.Unmarshal(data, &block); err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
		return blocks, nil
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}
		var blocks []AnthropicContentBlock
		if err := json.Unmarshal(data, &blocks); err != nil {
			return nil, err
		}
		return blocks, nil
	}
}

func copyMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func joinWithDoubleNewline(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "\n\n"
		}
		result += part
	}
	return result
}

func safeJSONString(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil {
		return value
	}
	return decoded
}

func stringify(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
}
