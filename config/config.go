package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// SourceConfig holds upstream API configuration
type SourceConfig struct {
	Protocol string
	Host     string
	Token    string
}

// APIConfig holds API server configuration
type APIConfig struct {
	Port      int
	Debug     bool
	DebugMsg  bool
	Think     string
	Anonymous bool
}

// ModelConfig holds model configuration
type ModelConfig struct {
	Default string
	Mapping map[string]string
}

// Config holds all configuration
type Config struct {
	Source  SourceConfig
	API     APIConfig
	Model   ModelConfig
	Headers map[string]string
}

var cfg *Config

// GetConfig returns the singleton config instance
func GetConfig() *Config {
	if cfg == nil {
		cfg = loadConfig()
	}
	return cfg
}

func loadConfig() *Config {
	// Load .env file (ignore error if not exists)
	_ = godotenv.Load()

	c := &Config{
		Source: SourceConfig{
			Protocol: "https:",
			Host:     "chat.z.ai",
			Token:    strings.TrimSpace(getEnv("TOKEN", "")),
		},
		API: APIConfig{
			Port:     getEnvInt("PORT", 8080),
			Debug:    getEnvBool("DEBUG", false),
			DebugMsg: getEnvBool("DEBUG_MSG", false),
			Think:    getEnv("THINK_TAGS_MODE", "reasoning"),
		},
		Model: ModelConfig{
			Default: getEnv("MODEL", "glm-4.6"),
			Mapping: make(map[string]string),
		},
		Headers: make(map[string]string),
	}

	// Set anonymous mode based on token presence
	c.API.Anonymous = (c.Source.Token == "")

	// Initialize headers
	c.initHeaders()

	// Validate configuration
	c.validate()

	return c
}

func (c *Config) initHeaders() {
	c.Headers = map[string]string{
		"Accept":             "*/*",
		"Accept-Language":    "en-US",
		"Cache-Control":      "no-cache",
		"Connection":         "keep-alive",
		"Pragma":             "no-cache",
		"Sec-Ch-Ua":          `"Microsoft Edge";v="141", "Not?A_Brand";v="8"`,
		"Sec-Ch-Ua-Mobile":   "?0",
		"Sec-Ch-Ua-Platform": "Linux",
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-origin",
		"User-Agent":         "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0",
		"X-FE-Version":       "prod-fe-1.0.117",
		"Origin":             c.Source.Protocol + "//" + c.Source.Host,
		"Referer":            c.Source.Protocol + "//" + c.Source.Host + "/",
	}
}

func (c *Config) validate() {
	// Validate think mode
	validThinkModes := []string{"reasoning", "think", "strip", "details"}
	valid := false
	for _, mode := range validThinkModes {
		if c.API.Think == mode {
			valid = true
			break
		}
	}
	if !valid {
		log.Printf("Warning: Invalid THINK_TAGS_MODE '%s', using 'reasoning'", c.API.Think)
		c.API.Think = "reasoning"
	}

	// Validate port
	if c.API.Port < 1 || c.API.Port > 65535 {
		log.Printf("Warning: Invalid PORT %d, using 8080", c.API.Port)
		c.API.Port = 8080
	}
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true"
	}
	return defaultValue
}