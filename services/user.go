package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/vantaidinh04/z2api-go/config"
	"github.com/vantaidinh04/z2api-go/types"
)

// UserService handles user authentication and caching
type UserService struct {
	cache map[string]*CachedUser
	mutex sync.RWMutex
}

// CachedUser represents a cached user with timestamp
type CachedUser struct {
	Info     *types.UserInfo
	CachedAt time.Time
}

var (
	userService     *UserService
	userServiceOnce sync.Once
)

// GetUserService returns the singleton user service instance
func GetUserService() *UserService {
	userServiceOnce.Do(func() {
		userService = &UserService{
			cache: make(map[string]*CachedUser),
		}
	})
	return userService
}

// GetUser gets user information with caching support
func (s *UserService) GetUser() (*types.UserInfo, error) {
	cfg := config.GetConfig()

	// Determine current token
	var currentToken string
	if cfg.API.Anonymous {
		currentToken = "" // Will fetch anonymous token
	} else {
		currentToken = cfg.Source.Token
	}

	// Check cache if token exists
	if currentToken != "" {
		s.mutex.RLock()
		cached, exists := s.cache[currentToken]
		s.mutex.RUnlock()

		if exists && time.Since(cached.CachedAt) < 30*time.Minute {
			log.Printf("User info [cached]: id=%s, token=%s...", cached.Info.ID, truncateString(currentToken, 50))
			return &types.UserInfo{
				ID:    cached.Info.ID,
				Name:  cached.Info.Name,
				Token: currentToken,
			}, nil
		}
	}

	// Fetch from API
	url := fmt.Sprintf("%s//%s/api/v1/auths/", cfg.Source.Protocol, cfg.Source.Host)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add authorization if not anonymous
	if !cfg.API.Anonymous && currentToken != "" {
		req.Header.Set("Authorization", "Bearer "+currentToken)
	}

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch user info failed: %s", string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	userName := getStringFromMap(result, "name")
	userID := getStringFromMap(result, "id")
	userToken := getStringFromMap(result, "token")

	// Use provided token if not anonymous
	if !cfg.API.Anonymous {
		userToken = currentToken
	}

	// Create user info
	userInfo := &types.UserInfo{
		ID:    userID,
		Name:  userName,
		Token: userToken,
	}

	// Cache result if token exists
	if userToken != "" && userID != "" {
		s.mutex.Lock()
		s.cache[userToken] = &CachedUser{
			Info: &types.UserInfo{
				ID:   userID,
				Name: userName,
			},
			CachedAt: time.Now(),
		}
		s.mutex.Unlock()
	}

	log.Printf("User info [live]: name=%s, id=%s, token=%s...", userName, userID, truncateString(userToken, 50))
	return userInfo, nil
}

// ClearCache clears the user cache
func (s *UserService) ClearCache() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.cache = make(map[string]*CachedUser)
	log.Println("User cache cleared")
}

// Helper functions

func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
