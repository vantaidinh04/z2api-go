package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Tyler-Dinh/z2api-go/config"
	"github.com/Tyler-Dinh/z2api-go/services"
	"github.com/Tyler-Dinh/z2api-go/utils"
)

// ChatCompletions handles OpenAI-compatible chat completions endpoint
func ChatCompletions(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse request body
	var requestData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": 400, "message": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	// Generate IDs
	chatID := utils.GenerateID()
	messageID := utils.GenerateID()

	// Get stream and include_usage options
	stream := false
	if s, ok := requestData["stream"].(bool); ok {
		stream = s
	}

	includeUsage := true
	if streamOpts, ok := requestData["stream_options"].(map[string]interface{}); ok {
		if iu, ok := streamOpts["include_usage"].(bool); ok {
			includeUsage = iu
		}
	}

	// Format request for Z.ai
	formattedData, err := services.FormatRequest(requestData, "OpenAI")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": 500, "message": "Failed to format request: %v"}`, err), http.StatusInternalServerError)
		return
	}

	formattedData["chat_id"] = chatID
	formattedData["id"] = messageID

	// Get model and messages
	cfg := config.GetConfig()
	model := cfg.Model.Default
	if m, ok := formattedData["model"].(string); ok && m != "" {
		model = m
	}

	messages := []map[string]interface{}{}
	if msgs, ok := formattedData["messages"].([]map[string]interface{}); ok {
		messages = msgs
	}

	// Calculate prompt tokens if needed
	promptTokens := 0
	if includeUsage {
		promptText := services.ExtractTextFromMessages(messages)
		promptTokens = services.CountTokens(promptText)
	}

	// Send request to Z.ai
	resp, err := services.SendChatRequest(formattedData, chatID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": 500, "message": "Failed to send request: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprintf(`{"error": %d, "message": "Z.ai API error"}`, resp.StatusCode), resp.StatusCode)
		return
	}

	// Handle streaming response
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, `{"error": 500, "message": "Streaming not supported"}`, http.StatusInternalServerError)
			return
		}

		completionParts := []string{}

		// Stream responses
		for zaiResp := range services.ParseSSEStream(resp) {
			delta := services.FormatResponse(zaiResp, "OpenAI")
			if delta == nil {
				continue
			}

			// Collect content for token counting
			if includeUsage {
				if content, ok := delta["content"].(string); ok {
					completionParts = append(completionParts, content)
				}
				if reasoningContent, ok := delta["reasoning_content"].(string); ok {
					completionParts = append(completionParts, reasoningContent)
				}
			}

			// Send chunk
			chunk := map[string]interface{}{
				"id":      utils.GenerateID(),
				"object":  "chat.completion.chunk",
				"created": time.Now().UnixMilli(),
				"model":   model,
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         delta,
						"message":       delta,
						"finish_reason": nil,
					},
				},
			}

			chunkJSON, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
			flusher.Flush()
		}

		// Send finish_reason
		finishChunk := map[string]interface{}{
			"id":      utils.GenerateID(),
			"object":  "chat.completion.chunk",
			"created": time.Now().UnixMilli(),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         map[string]interface{}{"role": "assistant"},
					"message":       map[string]interface{}{"role": "assistant"},
					"finish_reason": "stop",
				},
			},
		}
		finishJSON, _ := json.Marshal(finishChunk)
		fmt.Fprintf(w, "data: %s\n\n", finishJSON)
		flusher.Flush()

		// Send usage if requested
		if includeUsage {
			completionStr := strings.Join(completionParts, "")
			completionTokens := services.CountTokens(completionStr)

			usageChunk := map[string]interface{}{
				"id":      utils.GenerateID(),
				"object":  "chat.completion.chunk",
				"created": time.Now().UnixMilli(),
				"model":   model,
				"choices": []map[string]interface{}{},
				"usage": map[string]interface{}{
					"prompt_tokens":     promptTokens,
					"completion_tokens": completionTokens,
					"total_tokens":      promptTokens + completionTokens,
				},
			}
			usageJSON, _ := json.Marshal(usageChunk)
			fmt.Fprintf(w, "data: %s\n\n", usageJSON)
			flusher.Flush()
		}

		// Send [DONE]
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()

		return
	}

	// Handle non-streaming response
	contentParts := []string{}
	reasoningParts := []string{}

	for zaiResp := range services.ParseSSEStream(resp) {
		if zaiResp.Data != nil && zaiResp.Data.Done {
			break
		}

		delta := services.FormatResponse(zaiResp, "OpenAI")
		if delta == nil {
			continue
		}

		if content, ok := delta["content"].(string); ok {
			contentParts = append(contentParts, content)
		}
		if reasoningContent, ok := delta["reasoning_content"].(string); ok {
			reasoningParts = append(reasoningParts, reasoningContent)
		}
	}

	// Build final message
	finalMessage := map[string]interface{}{
		"role": "assistant",
	}
	completionStr := ""

	if len(reasoningParts) > 0 {
		reasoningText := strings.Join(reasoningParts, "")
		finalMessage["reasoning_content"] = reasoningText
		completionStr += reasoningText
	}
	if len(contentParts) > 0 {
		contentText := strings.Join(contentParts, "")
		finalMessage["content"] = contentText
		completionStr += contentText
	}

	// Build response
	result := map[string]interface{}{
		"id":      utils.GenerateID(),
		"object":  "chat.completion",
		"created": time.Now().UnixMilli(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"message":       finalMessage,
				"finish_reason": "stop",
			},
		},
	}

	// Add usage if requested
	if includeUsage {
		completionTokens := services.CountTokens(completionStr)
		result["usage"] = map[string]interface{}{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
