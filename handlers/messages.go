package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vantaidinh04/z2api-go/config"
	"github.com/vantaidinh04/z2api-go/services"
	"github.com/vantaidinh04/z2api-go/utils"
)

// AnthropicMessages handles Anthropic-compatible messages endpoint
func AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse request body
	var requestData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": {"type": "invalid_request_error", "message": "Invalid JSON: %v"}}`, err), http.StatusBadRequest)
		return
	}

	// Generate IDs
	chatID := utils.GenerateID()
	messageID := utils.GenerateID()

	// Get stream option
	stream := false
	if s, ok := requestData["stream"].(bool); ok {
		stream = s
	}

	// Format request for Z.ai
	formattedData, err := services.FormatRequest(requestData, "Anthropic")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": {"type": "api_error", "message": "Failed to format request: %v"}}`, err), http.StatusInternalServerError)
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

	// Calculate prompt tokens (required for Anthropic format)
	promptText := services.ExtractTextFromMessages(messages)
	promptTokens := services.CountTokens(promptText)

	// Send request to Z.ai
	resp, err := services.SendChatRequest(formattedData, chatID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": {"type": "api_error", "message": "Failed to send request: %v"}}`, err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprintf(`{"error": {"type": "api_error", "message": "Z.ai API error"}}`, resp.StatusCode), resp.StatusCode)
		return
	}

	// Handle streaming response
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, `{"error": {"type": "api_error", "message": "Streaming not supported"}}`, http.StatusInternalServerError)
			return
		}

		textParts := []string{}
		toolCallParts := []string{}
		hasToolCall := false

		// Send message_start event
		fmt.Fprintf(w, "event: message_start\n")
		messageStart := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":            utils.GenerateID(),
				"type":          "message",
				"role":          "assistant",
				"model":         model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]interface{}{
					"input_tokens":  promptTokens,
					"output_tokens": 0,
				},
			},
		}
		messageStartJSON, _ := json.Marshal(messageStart)
		fmt.Fprintf(w, "data: %s\n\n", messageStartJSON)
		flusher.Flush()

		// Send content_block_start event
		fmt.Fprintf(w, "event: content_block_start\n")
		contentBlockStart := map[string]interface{}{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		contentBlockStartJSON, _ := json.Marshal(contentBlockStart)
		fmt.Fprintf(w, "data: %s\n\n", contentBlockStartJSON)
		flusher.Flush()

		// Send ping event
		fmt.Fprintf(w, "event: ping\n")
		fmt.Fprintf(w, "data: {\"type\": \"ping\"}\n\n")
		flusher.Flush()

		// Stream responses
		for zaiResp := range services.ParseSSEStream(resp) {
			if zaiResp.Data != nil && zaiResp.Data.Done {
				break
			}

			delta := services.FormatResponse(zaiResp, "Anthropic")
			if delta == nil {
				continue
			}

			// Handle tool calls
			if toolCall, ok := delta["tool_call"].(string); ok {
				toolCallParts = append(toolCallParts, toolCall)
				toolCallStr := strings.Join(toolCallParts, "")

				// Try to parse complete tool call
				var toolJSON map[string]interface{}
				if err := json.Unmarshal([]byte(toolCallStr), &toolJSON); err == nil {
					// Process arguments -> input
					if arguments, ok := toolJSON["arguments"].(string); ok {
						var input map[string]interface{}
						if err := json.Unmarshal([]byte(arguments), &input); err == nil {
							toolJSON["input"] = input
						}
						delete(toolJSON, "arguments")
					}

					hasToolCall = true

					// Close text block
					fmt.Fprintf(w, "event: content_block_stop\n")
					fmt.Fprintf(w, "data: {\"type\": \"content_block_stop\", \"index\": 0}\n\n")
					flusher.Flush()

					// Start tool_use block
					fmt.Fprintf(w, "event: content_block_start\n")
					toolBlockStart := map[string]interface{}{
						"type":  "content_block_start",
						"index": 1,
						"content_block": map[string]interface{}{
							"type":  "tool_use",
							"id":    toolJSON["id"],
							"name":  toolJSON["name"],
							"input": nil,
						},
					}
					toolBlockStartJSON, _ := json.Marshal(toolBlockStart)
					fmt.Fprintf(w, "data: %s\n\n", toolBlockStartJSON)
					flusher.Flush()

					// Send input in chunks
					if input, ok := toolJSON["input"].(map[string]interface{}); ok {
						inputJSON, _ := json.Marshal(input)
						inputStr := string(inputJSON)
						chunkSize := 5

						for i := 0; i < len(inputStr); i += chunkSize {
							end := i + chunkSize
							if end > len(inputStr) {
								end = len(inputStr)
							}
							chunk := inputStr[i:end]

							fmt.Fprintf(w, "event: content_block_delta\n")
							delta := map[string]interface{}{
								"type":  "content_block_delta",
								"index": 1,
								"delta": map[string]interface{}{
									"type":         "input_json_delta",
									"partial_json": chunk,
								},
							}
							deltaJSON, _ := json.Marshal(delta)
							fmt.Fprintf(w, "data: %s\n\n", deltaJSON)
							flusher.Flush()
						}
					}

					// Close tool_use block
					fmt.Fprintf(w, "event: content_block_stop\n")
					fmt.Fprintf(w, "data: {\"type\": \"content_block_stop\", \"index\": 1}\n\n")
					flusher.Flush()
					break
				}
				continue
			}

			// Handle text content
			if text, ok := delta["text"].(string); ok {
				textParts = append(textParts, text)

				fmt.Fprintf(w, "event: content_block_delta\n")
				contentDelta := map[string]interface{}{
					"type":  "content_block_delta",
					"index": 0,
					"delta": map[string]interface{}{
						"type": "text_delta",
						"text": text,
					},
				}
				contentDeltaJSON, _ := json.Marshal(contentDelta)
				fmt.Fprintf(w, "data: %s\n\n", contentDeltaJSON)
				flusher.Flush()
			}
		}

		// Calculate completion tokens
		completionStr := strings.Join(textParts, "")
		completionTokens := services.CountTokens(completionStr)

		// Close text block if no tool call
		if !hasToolCall {
			fmt.Fprintf(w, "event: content_block_stop\n")
			fmt.Fprintf(w, "data: {\"type\": \"content_block_stop\", \"index\": 0}\n\n")
			flusher.Flush()
		}

		// Send message_delta event
		fmt.Fprintf(w, "event: message_delta\n")
		stopReason := "end_turn"
		if hasToolCall {
			stopReason = "tool_use"
		}
		messageDelta := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": map[string]interface{}{
				"output_tokens": completionTokens,
			},
		}
		messageDeltaJSON, _ := json.Marshal(messageDelta)
		fmt.Fprintf(w, "data: %s\n\n", messageDeltaJSON)
		flusher.Flush()

		// Send message_stop event
		fmt.Fprintf(w, "event: message_stop\n")
		fmt.Fprintf(w, "data: {\"type\": \"message_stop\"}\n\n")
		flusher.Flush()

		return
	}

	// Handle non-streaming response
	textParts := []string{}
	toolCallParts := []string{}

	for zaiResp := range services.ParseSSEStream(resp) {
		if zaiResp.Data != nil && zaiResp.Data.Done {
			break
		}

		delta := services.FormatResponse(zaiResp, "Anthropic")
		if delta == nil {
			continue
		}

		if toolCall, ok := delta["tool_call"].(string); ok {
			toolCallParts = append(toolCallParts, toolCall)
			toolCallStr := strings.Join(toolCallParts, "")

			// Try to parse complete tool call
			var toolJSON map[string]interface{}
			if err := json.Unmarshal([]byte(toolCallStr), &toolJSON); err == nil {
				// Process arguments -> input
				if arguments, ok := toolJSON["arguments"].(string); ok {
					var input map[string]interface{}
					if err := json.Unmarshal([]byte(arguments), &input); err == nil {
						toolJSON["input"] = input
					}
					delete(toolJSON, "arguments")
				}

				completionTokens := services.CountTokens(strings.Join(textParts, ""))

				// Build content array
				content := []map[string]interface{}{}
				if len(textParts) > 0 {
					content = append(content, map[string]interface{}{
						"type": "text",
						"text": strings.Join(textParts, ""),
					})
				}
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    toolJSON["id"],
					"name":  toolJSON["name"],
					"input": toolJSON["input"],
				})

				result := map[string]interface{}{
					"id":      utils.GenerateID(),
					"type":    "message",
					"role":    "assistant",
					"model":   model,
					"content": content,
					"usage": map[string]interface{}{
						"input_tokens":  promptTokens,
						"output_tokens": completionTokens,
					},
					"stop_sequence": nil,
					"stop_reason":   "tool_use",
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
				return
			}
			continue
		}

		if content, ok := delta["content"].(string); ok {
			textParts = append(textParts, content)
		}
		if reasoningContent, ok := delta["reasoning_content"].(string); ok {
			textParts = append(textParts, reasoningContent)
		}
	}

	// No tool call, pure text response
	completionStr := strings.Join(textParts, "")
	completionTokens := services.CountTokens(completionStr)

	content := []map[string]interface{}{}
	if completionStr != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": completionStr,
		})
	}

	result := map[string]interface{}{
		"id":      utils.GenerateID(),
		"type":    "message",
		"role":    "assistant",
		"model":   model,
		"content": content,
		"usage": map[string]interface{}{
			"input_tokens":  promptTokens,
			"output_tokens": completionTokens,
		},
		"stop_sequence": nil,
		"stop_reason":   "end_turn",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
