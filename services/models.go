package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Tyler-Dinh/z2api-go/config"
	"github.com/Tyler-Dinh/z2api-go/types"
)

// ModelsService handles model listing and mapping
type ModelsService struct {
	cache      *types.ModelsResponse
	cacheMutex sync.RWMutex
}

var (
	modelsService     *ModelsService
	modelsServiceOnce sync.Once
)

// GetModelsService returns the singleton models service instance
func GetModelsService() *ModelsService {
	modelsServiceOnce.Do(func() {
		modelsService = &ModelsService{}
	})
	return modelsService
}

// GetModels fetches and caches the model list from Z.ai API
func (s *ModelsService) GetModels() (*types.ModelsResponse, error) {
	cfg := config.GetConfig()

	// Check cache
	s.cacheMutex.RLock()
	if s.cache != nil {
		s.cacheMutex.RUnlock()
		return s.cache, nil
	}
	s.cacheMutex.RUnlock()

	// Get user token
	userService := GetUserService()
	user, err := userService.GetUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	currentToken := user.Token
	if !cfg.API.Anonymous {
		currentToken = cfg.Source.Token
	}

	// Fetch models from API
	url := fmt.Sprintf("%s//%s/api/models", cfg.Source.Protocol, cfg.Source.Host)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+currentToken)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch models failed: %s", string(body))
	}

	// Parse response
	var rawResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Process models
	models := []types.Model{}
	rawModels, ok := rawResponse["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	modelLogo := "data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2030%2030%22%20style%3D%22background%3A%232D2D2D%22%3E%3Cpath%20fill%3D%22%23FFFFFF%22%20d%3D%22M15.47%207.1l-1.3%201.85c-.2.29-.54.47-.9.47h-7.1V7.09c0%20.01%209.31.01%209.31.01z%22%2F%3E%3Cpath%20fill%3D%22%23FFFFFF%22%20d%3D%22M24.3%207.1L13.14%2022.91H5.7l11.16-15.81z%22%2F%3E%3Cpath%20fill%3D%22%23FFFFFF%22%20d%3D%22M14.53%2022.91l1.31-1.86c.2-.29.54-.47.9-.47h7.09v2.33h-9.3z%22%2F%3E%3C%2Fsvg%3E"

	for _, rawModel := range rawModels {
		modelMap, ok := rawModel.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if model is active
		info, _ := modelMap["info"].(map[string]interface{})
		if isActive, ok := info["is_active"].(bool); ok && !isActive {
			continue
		}

		sourceID := getStringFromInterface(modelMap["id"])
		modelName := getStringFromInterface(modelMap["name"])

		// Get model metadata
		meta, _ := info["meta"].(map[string]interface{})
		capabilities, _ := meta["capabilities"].(map[string]interface{})
		description := getStringFromInterface(meta["description"])
		hidden, _ := meta["hidden"].(bool)

		// Process suggestion prompts
		suggestionPrompts := []map[string]interface{}{}
		if prompts, ok := meta["suggestion_prompts"].([]interface{}); ok {
			for _, p := range prompts {
				if promptMap, ok := p.(map[string]interface{}); ok {
					if prompt, ok := promptMap["prompt"].(string); ok {
						suggestionPrompts = append(suggestionPrompts, map[string]interface{}{
							"content": prompt,
						})
					}
				}
			}
		}

		// Format metadata
		modelMeta := map[string]interface{}{
			"profile_image_url":  modelLogo,
			"capabilities":       capabilities,
			"description":        description,
			"hidden":             hidden,
			"suggestion_prompts": suggestionPrompts,
		}

		// Get formatted model name and ID
		formattedName := getModelName(sourceID, modelName)
		modelID := getModelID(sourceID, formattedName, cfg)

		// Get created timestamp
		var created int64
		if createdAt, ok := info["created_at"].(float64); ok {
			created = int64(createdAt)
		} else {
			created = time.Now().Unix()
		}

		// Create model
		model := types.Model{
			ID:      modelID,
			Object:  "model",
			Name:    formattedName,
			Meta:    modelMeta,
			Info:    map[string]interface{}{"meta": modelMeta},
			Created: created,
			OwnedBy: "z.ai",
			Original: map[string]interface{}{
				"name": modelName,
				"id":   sourceID,
				"info": info,
			},
			AccessControl: nil,
		}

		models = append(models, model)
	}

	// Create response
	result := &types.ModelsResponse{
		Object: "list",
		Data:   models,
	}

	// Cache result
	s.cacheMutex.Lock()
	s.cache = result
	s.cacheMutex.Unlock()

	log.Printf("Fetched %d models from Z.ai API", len(models))
	return result, nil
}

// ClearCache clears the models cache
func (s *ModelsService) ClearCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache = nil
	log.Println("Models cache cleared")
}

// Helper functions for model name formatting
func formatModelName(name string) string {
	if name == "" {
		return ""
	}

	parts := strings.Split(name, "-")
	if len(parts) == 1 {
		return strings.ToUpper(parts[0])
	}

	formatted := []string{strings.ToUpper(parts[0])}
	for _, p := range parts[1:] {
		if p == "" {
			formatted = append(formatted, "")
		} else if isDigit(p) {
			formatted = append(formatted, p)
		} else if hasAlpha(p) {
			formatted = append(formatted, strings.Title(p))
		} else {
			formatted = append(formatted, p)
		}
	}

	return strings.Join(formatted, "-")
}

// getModelName gets the display name for a model
func getModelName(sourceID, modelName string) string {
	// Handle models with built-in series names
	if (strings.HasPrefix(sourceID, "GLM") || strings.HasPrefix(sourceID, "Z")) && strings.Contains(sourceID, ".") {
		return sourceID
	}

	if (strings.HasPrefix(modelName, "GLM") || strings.HasPrefix(modelName, "Z")) && strings.Contains(modelName, ".") {
		return modelName
	}

	// If name doesn't start with letter, format from source ID
	if modelName == "" || !isLetter(rune(modelName[0])) {
		modelName = formatModelName(sourceID)
		if !strings.HasPrefix(strings.ToUpper(modelName), "GLM") && !strings.HasPrefix(strings.ToUpper(modelName), "Z") {
			modelName = "GLM-" + formatModelName(sourceID)
		}
	}

	return modelName
}

// getModelID gets the API ID for a model
func getModelID(sourceID, modelName string, cfg *config.Config) string {
	// Check if mapping exists
	if mappedID, ok := cfg.Model.Mapping[sourceID]; ok {
		return mappedID
	}

	// Generate smart ID
	smartID := strings.ToLower(modelName)
	cfg.Model.Mapping[sourceID] = smartID
	return smartID
}

// Helper functions

func getStringFromInterface(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func isDigit(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func hasAlpha(s string) bool {
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

func isLetter(c rune) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
