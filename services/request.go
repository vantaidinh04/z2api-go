package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tyler-Dinh/z2api-go/config"
	"github.com/Tyler-Dinh/z2api-go/types"
	"github.com/Tyler-Dinh/z2api-go/utils"
)

// FormatRequest converts OpenAI/Anthropic format to Z.ai format
func FormatRequest(data map[string]interface{}, requestType string) (map[string]interface{}, error) {
	cfg := config.GetConfig()
	result := make(map[string]interface{})

	// Copy original data
	for k, v := range data {
		result[k] = v
	}

	// Get model
	model, ok := result["model"].(string)
	if !ok || model == "" {
		model = cfg.Model.Default
	}

	// Get chat_id
	chatID, _ := result["chat_id"].(string)

	// Process messages
	newMessages := []map[string]interface{}{}

	// Handle Anthropic system parameter
	if system, ok := result["system"]; ok {
		var content string
		switch v := system.(type) {
		case string:
			content = strings.TrimLeft(v, "\n")
		case []interface{}:
			items := []string{}
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemMap["type"] == "text" {
						if text, ok := itemMap["text"].(string); ok {
							items = append(items, strings.TrimLeft(text, "\n"))
						}
					}
				}
			}
			content = strings.Join(items, "\n\n")
		}
		if content != "" {
			newMessages = append(newMessages, map[string]interface{}{
				"role":    "system",
				"content": content,
			})
		}
		delete(result, "system")
	}

	// Process messages array
	if messages, ok := result["messages"].([]interface{}); ok {
		for _, msg := range messages {
			message, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}

			role, _ := message["role"].(string)
			content := message["content"]
			newMessage := map[string]interface{}{"role": role}

			// Handle string content
			if contentStr, ok := content.(string); ok {
				newMessage["content"] = contentStr
				newMessages = append(newMessages, newMessage)
				continue
			}

			// Handle array content
			if contentArr, ok := content.([]interface{}); ok {
				dontAppend := false
				var newContent interface{} = ""

				for _, item := range contentArr {
					itemMap, ok := item.(map[string]interface{})
					if !ok {
						continue
					}

					itemType, _ := itemMap["type"].(string)

					// Text content
					if itemType == "text" {
						if text, ok := itemMap["text"].(string); ok {
							newContent = text
						}
						continue
					}

					// Image content
					if itemType == "image_url" || itemType == "image" {
						mediaURL := ""

						// OpenAI format
						if imageURL, ok := itemMap["image_url"].(map[string]interface{}); ok {
							if urlStr, ok := imageURL["url"].(string); ok {
								mediaURL = urlStr
							}
						}

						// Anthropic format
						if source, ok := itemMap["source"].(map[string]interface{}); ok {
							if source["type"] == "base64" {
								if data, ok := source["data"].(string); ok {
									mediaType, _ := source["media_type"].(string)
									if mediaType == "" {
										mediaType = "image/jpeg"
									}
									mediaURL = fmt.Sprintf("data:%s;base64,%s", mediaType, data)
								}
							}
						}

						if mediaURL == "" {
							// Convert newContent to array if needed
							if contentStr, ok := newContent.(string); ok {
								newContent = []map[string]interface{}{
									{"type": "text", "text": contentStr},
								}
							}
							if contentSlice, ok := newContent.([]map[string]interface{}); ok {
								contentSlice = append(contentSlice, map[string]interface{}{
									"type": "text",
									"text": "system: image error - Unsupported format or missing URL",
								})
								newContent = contentSlice
							}
							continue
						}

						// Upload image if it's base64
						uploadedURL, err := UploadImage(mediaURL, chatID)
						if err != nil {
							// Convert newContent to array if needed
							if contentStr, ok := newContent.(string); ok {
								newContent = []map[string]interface{}{
									{"type": "text", "text": contentStr},
								}
							}
							if contentSlice, ok := newContent.([]map[string]interface{}); ok {
								contentSlice = append(contentSlice, map[string]interface{}{
									"type": "text",
									"text": fmt.Sprintf("system: image upload error - %v", err),
								})
								newContent = contentSlice
							}
							continue
						}
						if uploadedURL != "" {
							mediaURL = uploadedURL
						}

						// Convert newContent to array if needed
						if contentStr, ok := newContent.(string); ok {
							newContent = []map[string]interface{}{
								{"type": "text", "text": contentStr},
							}
						}
						if contentSlice, ok := newContent.([]map[string]interface{}); ok {
							contentSlice = append(contentSlice, map[string]interface{}{
								"type":      "image_url",
								"image_url": map[string]interface{}{"url": mediaURL},
							})
							newContent = contentSlice
						}
					}

					// Anthropic tool_use
					if itemType == "tool_use" && role == "assistant" {
						if newMessage["tool_calls"] == nil {
							newMessage["tool_calls"] = []map[string]interface{}{}
						}

						toolCalls := newMessage["tool_calls"].([]map[string]interface{})
						arguments := "{}"
						if input, ok := itemMap["input"].(map[string]interface{}); ok {
							if argBytes, err := json.Marshal(input); err == nil {
								arguments = string(argBytes)
							}
						}

						toolCalls = append(toolCalls, map[string]interface{}{
							"id":   itemMap["id"],
							"type": "function",
							"function": map[string]interface{}{
								"name":      itemMap["name"],
								"arguments": arguments,
							},
						})
						newMessage["tool_calls"] = toolCalls
						dontAppend = true
					}

					// Anthropic tool_result
					if itemType == "tool_result" {
						toolResultContent := itemMap["content"]
						result := ""

						if contentArr, ok := toolResultContent.([]interface{}); ok {
							parts := []string{}
							for _, c := range contentArr {
								if cMap, ok := c.(map[string]interface{}); ok {
									if cMap["type"] == "text" {
										if text, ok := cMap["text"].(string); ok && text != "" {
											parts = append(parts, text)
										}
									}
								}
							}
							if len(parts) > 0 {
								result = strings.Join(parts, "")
							}
						} else if resultStr, ok := toolResultContent.(string); ok {
							result = resultStr
						}

						newMessages = append(newMessages, map[string]interface{}{
							"role":         "tool",
							"tool_call_id": itemMap["tool_use_id"],
							"content":      result,
						})
						dontAppend = true
					}
				}

				if !dontAppend {
					newMessage["content"] = newContent
					newMessages = append(newMessages, newMessage)
				}
			}
		}
	}

	// Reverse model mapping (user-friendly ID -> source ID)
	modelsService := GetModelsService()
	models, _ := modelsService.GetModels()
	if models != nil && models.Data != nil {
		for _, m := range models.Data {
			if m.ID == model && m.Original != nil {
				if sourceID, ok := m.Original["id"].(string); ok && model != sourceID {
					model = sourceID
					break
				}
			}
		}
	}

	result["model"] = model
	result["messages"] = newMessages
	result["stream"] = true

	// Handle features
	features := map[string]interface{}{
		"enable_thinking": false,
	}
	if existingFeatures, ok := result["features"].(map[string]interface{}); ok {
		for k, v := range existingFeatures {
			features[k] = v
		}
	}

	// Handle enable_thinking from Qwen format
	if enableThinking, ok := result["enable_thinking"]; ok {
		features["enable_thinking"] = enableThinking
		delete(result, "enable_thinking")
	}

	// Handle thinking from Anthropic/CherryStudio format
	if thinking, ok := result["thinking"].(map[string]interface{}); ok {
		if thinkType, ok := thinking["type"].(string); ok {
			features["enable_thinking"] = strings.ToLower(thinkType) == "enabled"
		}
		delete(result, "thinking")
	}

	// Check if model supports thinking
	if models != nil && models.Data != nil {
		for _, m := range models.Data {
			if m.ID == model || (m.Original != nil && m.Original["id"] == model) {
				if m.Original != nil {
					if info, ok := m.Original["info"].(map[string]interface{}); ok {
						if meta, ok := info["meta"].(map[string]interface{}); ok {
							if caps, ok := meta["capabilities"].(map[string]interface{}); ok {
								if think, ok := caps["think"].(bool); ok && !think {
									delete(features, "enable_thinking")
								}
							}
						}
					}
				}
				break
			}
		}
	}

	if len(features) > 0 {
		result["features"] = features
	}

	return result, nil
}

// SendChatRequest sends a chat request to Z.ai API
func SendChatRequest(data map[string]interface{}, chatID string) (*http.Response, error) {
	cfg := config.GetConfig()
	timestamp := time.Now().UnixMilli()
	requestID := utils.GenerateID()

	// Get user info
	userService := GetUserService()
	user, err := userService.GetUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	userToken := user.Token
	userID := user.ID

	// Build query parameters
	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", timestamp))
	params.Set("requestId", requestID)

	// Build headers
	headers := make(map[string]string)
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	headers["Authorization"] = fmt.Sprintf("Bearer %s", userToken)
	headers["Content-Type"] = "application/json"
	headers["Referer"] = fmt.Sprintf("%s//%s/c/%s", cfg.Source.Protocol, cfg.Source.Host, chatID)

	// Add signature if user is authenticated
	if userID != "" {
		params.Set("user_id", userID)

		// Get last user message for signature
		lastUserMessage := ""
		if messages, ok := data["messages"].([]map[string]interface{}); ok {
			for _, msg := range messages {
				if role, ok := msg["role"].(string); ok && role == "user" {
					if content, ok := msg["content"].(string); ok {
						lastUserMessage = content
					} else if contentArr, ok := msg["content"].([]interface{}); ok {
						texts := []string{}
						for _, item := range contentArr {
							if itemMap, ok := item.(map[string]interface{}); ok {
								if itemMap["type"] == "text" {
									if text, ok := itemMap["text"].(string); ok {
										texts = append(texts, text)
										break
									}
								}
							}
						}
						lastUserMessage = strings.Join(texts, "")
					}
				}
			}
		}

		// Generate signature
		sigParams := map[string]string{
			"requestId": requestID,
			"timestamp": fmt.Sprintf("%d", timestamp),
			"user_id":   userID,
		}
		sigResult, err := GenerateSignature(sigParams, lastUserMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to generate signature: %w", err)
		}
		headers["X-Signature"] = sigResult.Signature
		params.Set("signature_timestamp", fmt.Sprintf("%d", sigResult.Timestamp))
		data["signature_prompt"] = lastUserMessage
	}

	// Build URL
	apiURL := fmt.Sprintf("%s//%s/api/chat/completions?%s",
		cfg.Source.Protocol, cfg.Source.Host, params.Encode())

	// Marshal request body
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Send request
	client := &http.Client{
		Timeout: 0, // No timeout for streaming
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// UploadImage uploads a base64 image to Z.ai API
func UploadImage(dataURL, chatID string) (string, error) {
	cfg := config.GetConfig()

	// Skip upload in anonymous mode or if not base64
	if cfg.API.Anonymous || !strings.HasPrefix(dataURL, "data:") {
		return "", nil
	}

	// Parse data URL
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data URL format")
	}

	header := parts[0]
	encoded := parts[1]

	// Extract MIME type (not used but kept for potential future use)
	_ = header // Suppress unused variable warning

	// Decode base64
	imageData, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Generate filename
	filename := utils.GenerateID()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}
	writer.Close()

	// Get user token
	userService := GetUserService()
	user, err := userService.GetUser()
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}

	// Build request
	uploadURL := fmt.Sprintf("%s//%s/api/v1/files/", cfg.Source.Protocol, cfg.Source.Host)
	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}

	// Set headers
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.Token))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Referer", fmt.Sprintf("%s//%s/c/%s", cfg.Source.Protocol, cfg.Source.Host, chatID))

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result types.ImageUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse upload response: %w", err)
	}

	return fmt.Sprintf("%s_%s", result.ID, result.Filename), nil
}
