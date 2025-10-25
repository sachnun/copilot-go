package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"internal/approval"
	"internal/logger"
	"internal/messages"
	"internal/rate"
	"internal/services/copilot"
	"internal/services/github"
	"internal/state"
	"internal/streaming"
)

type Server struct {
	state    *state.State
	client   *http.Client
	streamer copilot.SSEReader
	mux      *http.ServeMux
}

func New(s *state.State, client *http.Client) *Server {
	if client == nil {
		client = http.DefaultClient
	}

	srv := &Server{
		state:    s,
		client:   client,
		streamer: streaming.Reader{},
		mux:      http.NewServeMux(),
	}

	srv.routes()
	return srv
}

func (s *Server) Handler() http.Handler {
	return Chain(s.mux, LoggingMiddleware, CORSMiddleware)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.Handle("/chat/completions", Chain(http.HandlerFunc(s.handleChatCompletions), APIKeyMiddleware))
	s.mux.Handle("/v1/chat/completions", Chain(http.HandlerFunc(s.handleChatCompletions), APIKeyMiddleware))

	s.mux.Handle("/embeddings", Chain(http.HandlerFunc(s.handleEmbeddings), APIKeyMiddleware))
	s.mux.Handle("/v1/embeddings", Chain(http.HandlerFunc(s.handleEmbeddings), APIKeyMiddleware))

	s.mux.Handle("/models", http.HandlerFunc(s.handleModels))
	s.mux.Handle("/v1/models", http.HandlerFunc(s.handleModels))

	s.mux.Handle("/usage", http.HandlerFunc(s.handleUsage))

	s.mux.Handle("/responses", Chain(http.HandlerFunc(s.handleResponses), APIKeyMiddleware))
	s.mux.Handle("/v1/responses", Chain(http.HandlerFunc(s.handleResponses), APIKeyMiddleware))

	s.mux.Handle("/v1/messages", Chain(http.HandlerFunc(s.handleMessages), APIKeyMiddleware))
	s.mux.Handle("/v1/messages/count_tokens", Chain(http.HandlerFunc(s.handleMessagesCountTokens), APIKeyMiddleware))
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	var started *int64
	s.state.Read(func(st *state.State) {
		started = st.ServerStartUnixMs
	})

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if started != nil {
		uptime := time.Now().UnixMilli() - *started
		w.Write([]byte("Already running since " + formatUptime(uptime) + " ago"))
		return
	}
	w.Write([]byte("Server running"))
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if err := rate.CheckRateLimit(s.state); err != nil {
		writeError(w, err)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	logger.Debug("Chat completion request payload (last 400 bytes): %s", truncateBody(body, 400))

	var payload copilot.ChatCompletionsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, err)
		return
	}

	if manual := s.manualApprove(); manual {
		if err := approval.AwaitApproval(); err != nil {
			writeError(w, err)
			return
		}
	}

	stream := payload.Stream != nil && *payload.Stream
	if stream {
		logger.Debug("Streaming chat completion for model %s", payload.Model)
	} else {
		logger.Debug("Non-streaming chat completion for model %s", payload.Model)
	}

	result, err := copilot.CreateChatCompletions(r.Context(), s.state, payload, s.client, s.streamer)
	if err != nil {
		writeError(w, err)
		return
	}

	if stream {
		s.forwardStream(w, r, result)
		return
	}

	logger.Debug("Chat completion response ready (non-streaming)")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) manualApprove() bool {
	var manual bool
	s.state.Read(func(st *state.State) {
		manual = st.ManualApprove
	})
	return manual
}

func (s *Server) forwardStream(w http.ResponseWriter, r *http.Request, stream interface{}) {
	messageChan, ok := stream.(<-chan copilot.SSEMessage)
	if !ok {
		writeError(w, fmt.Errorf("invalid stream type"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming unsupported by server"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageChan:
			if !ok {
				return
			}
			if msg.Event != "" {
				fmt.Fprintf(w, "event: %s\n", msg.Event)
			}
			logger.Debug("Streaming chat chunk event=%s size=%d", msg.Event, len(msg.Data))
			fmt.Fprintf(w, "data: %s\n\n", msg.Data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if err := rate.CheckRateLimit(s.state); err != nil {
		writeError(w, err)
		return
	}

	var payload copilot.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, err)
		return
	}

	if manual := s.manualApprove(); manual {
		if err := approval.AwaitApproval(); err != nil {
			writeError(w, err)
			return
		}
	}

	result, err := copilot.CreateEmbeddings(r.Context(), s.state, s.client, payload)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	var cached *copilot.ModelsResponse
	s.state.Read(func(st *state.State) {
		if models, ok := st.Models.(*copilot.ModelsResponse); ok {
			cached = models
		}
	})

	if cached != nil {
		s.writeModels(w, cached)
		return
	}

	models, err := copilot.GetModels(r.Context(), s.state, s.client)
	if err != nil {
		writeError(w, err)
		return
	}

	s.state.Update(func(st *state.State) {
		st.Models = models
	})

	s.writeModels(w, models)
}

func (s *Server) writeModels(w http.ResponseWriter, models *copilot.ModelsResponse) {
	data := make([]map[string]any, 0, len(models.Data))
	for _, model := range models.Data {
		data = append(data, map[string]any{
			"id":           model.ID,
			"object":       "model",
			"type":         "model",
			"created":      0,
			"created_at":   time.Unix(0, 0).UTC().Format(time.RFC3339),
			"owned_by":     model.Vendor,
			"display_name": model.Name,
		})
	}

	response := map[string]any{
		"object":   "list",
		"data":     data,
		"has_more": false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := github.GetCopilotUsage(r.Context(), s.state, s.client)
	if err != nil {
		logger.Error("Error fetching Copilot usage: %v", err)
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usage)
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if err := rate.CheckRateLimit(s.state); err != nil {
		writeError(w, err)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	logger.Debug("Messages request payload (last 400 bytes): %s", truncateBody(body, 400))

	var payload messages.AnthropicMessagesPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, err)
		return
	}

	if s.manualApprove() {
		if err := approval.AwaitApproval(); err != nil {
			writeError(w, err)
			return
		}
	}

	openaiPayload, err := messages.TranslateToOpenAI(payload)
	if err != nil {
		writeError(w, err)
		return
	}

	result, err := copilot.CreateChatCompletions(r.Context(), s.state, openaiPayload, s.client, s.streamer)
	if err != nil {
		writeError(w, err)
		return
	}

	streamRequested := payload.Stream != nil && *payload.Stream

	if streamRequested {
		logger.Debug("Streaming messages response for model %s", payload.Model)
		s.forwardMessagesStream(w, r, result)
		return
	}

	completion, ok := result.(copilot.ChatCompletionResponse)
	if !ok {
		writeError(w, fmt.Errorf("unexpected response type"))
		return
	}

	anthropic, err := messages.TranslateToAnthropic(completion)
	if err != nil {
		writeError(w, err)
		return
	}

	logger.Debug("Messages response completed (non-streaming)")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(anthropic); err != nil {
		logger.Error("Failed to write response: %v", err)
	}
}

func (s *Server) forwardMessagesStream(w http.ResponseWriter, r *http.Request, result interface{}) {
	ch, ok := result.(<-chan copilot.SSEMessage)
	if !ok {
		writeError(w, fmt.Errorf("invalid stream type"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming unsupported by server"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ctx := r.Context()
	streamState := messages.NewStreamState()

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-ch:
			if !ok {
				return
			}
			if chunk.Data == "[DONE]" {
				logger.Debug("Messages stream finished with [DONE]")
				return
			}

			if chunk.Data == "" {
				continue
			}

			var parsed copilot.ChatCompletionChunk
			if err := json.Unmarshal([]byte(chunk.Data), &parsed); err != nil {
				logger.Debug("Failed to decode stream chunk: %v", err)
				stop := messages.TranslateStreamError()
				fmt.Fprintf(w, "event: %s\n", stop.Type)
				fmt.Fprintf(w, "data: %s\n\n", stop.Data)
				flusher.Flush()
				return
			}

			events, err := messages.TranslateChunkToAnthropicEvents(parsed, &streamState)
			if err != nil {
				logger.Debug("Failed to translate stream chunk: %v", err)
				stop := messages.TranslateStreamError()
				fmt.Fprintf(w, "event: %s\n", stop.Type)
				fmt.Fprintf(w, "data: %s\n\n", stop.Data)
				flusher.Flush()
				return
			}
			for _, event := range events {
				logger.Debug("Forwarding messages stream event=%s", event.Type)
				fmt.Fprintf(w, "event: %s\n", event.Type)
				fmt.Fprintf(w, "data: %s\n\n", event.Data)
				flusher.Flush()
			}
		}
	}
}

func (s *Server) handleMessagesCountTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"input_tokens":1}`))
}

func truncateBody(body []byte, limit int) string {
	if len(body) <= limit {
		return string(body)
	}
	return string(body[len(body)-limit:])
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if err := rate.CheckRateLimit(s.state); err != nil {
		writeError(w, err)
		return
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}

	var payload copilot.ResponsesPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		writeError(w, err)
		return
	}

	if manual := s.manualApprove(); manual {
		if err := approval.AwaitApproval(); err != nil {
			writeError(w, err)
			return
		}
	}

	streamRequested := payload.StreamEnabled()

	var models *copilot.ModelsResponse
	s.state.Read(func(st *state.State) {
		if cached, ok := st.Models.(*copilot.ModelsResponse); ok {
			models = cached
		}
	})
	if models != nil {
		found := false
		for _, model := range models.Data {
			if model.ID == payload.Model {
				for _, endpoint := range model.SupportedEndpoints {
					if endpoint == "/responses" {
						found = true
						break
					}
				}
				break
			}
		}
		if !found {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "This model does not support the responses endpoint. Please choose a different model.",
					"type":    "invalid_request_error",
				},
			})
			return
		}
	}

	messages := extractMessagesFromResponses(payload)
	initiator := copilot.ResolveChatInitiator(payload.Model, messages)
	vision := copilot.HasVisionInput(payload)

	result, err := copilot.CreateResponses(r.Context(), s.state, rawBody, copilot.ResponsesRequestOptions{
		Vision:    vision,
		Initiator: initiator,
		Stream:    streamRequested,
	}, s.client, s.streamer)
	if err != nil {
		writeError(w, err)
		return
	}

	if streamRequested {
		s.forwardStream(w, r, result)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func extractMessagesFromResponses(payload copilot.ResponsesPayload) []copilot.Message {
	var messages []copilot.Message
	for _, item := range payload.InputItems() {
		if entry, ok := item.(map[string]any); ok {
			if role, ok := entry["role"].(string); ok {
				messages = append(messages, copilot.Message{Role: role})
			}
		}
	}
	return messages
}

func formatUptime(milliseconds int64) string {
	seconds := milliseconds / 1000
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	if days > 0 {
		remainingHours := hours % 24
		return fmt.Sprintf("%dd %dh", days, remainingHours)
	}
	if hours > 0 {
		remainingMinutes := minutes % 60
		return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
	}
	if minutes > 0 {
		remainingSeconds := seconds % 60
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
