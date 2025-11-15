package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Tyler-Dinh/z2api-go/config"
	"github.com/Tyler-Dinh/z2api-go/handlers"
	"github.com/Tyler-Dinh/z2api-go/middleware"
	"github.com/Tyler-Dinh/z2api-go/utils"
)

func main() {
	// Initialize configuration
	cfg := config.GetConfig()

	// Initialize tokenizer
	if err := utils.InitTokenizer(); err != nil {
		log.Printf("Warning: Failed to initialize tokenizer: %v", err)
		log.Println("Token counting will not be available")
	}

	// Setup routes
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", handlers.HealthHandler)
	mux.HandleFunc("/v1/models", handlers.ModelsHandler)
	mux.HandleFunc("/v1/chat/completions", handlers.ChatCompletions)
	mux.HandleFunc("/v1/messages", handlers.AnthropicMessages)

	// Apply CORS middleware
	handler := middleware.CORS(mux)

	// Print startup info
	log.Println("---------------------------------------------------------------------")
	log.Println("Z2api Go - OpenAI/Anthropic Compatible Proxy for Z.ai")
	log.Println("https://github.com/Tyler-Dinh/z2api-go")
	log.Println("---------------------------------------------------------------------")
	log.Printf("Base:           %s//%s", cfg.Source.Protocol, cfg.Source.Host)
	log.Printf("Port:           %d", cfg.API.Port)
	log.Printf("Think Mode:     %s", cfg.API.Think)
	log.Printf("Anonymous Mode: %v", cfg.API.Anonymous)
	log.Printf("Debug Mode:     %v", cfg.API.Debug)
	log.Printf("Debug Messages: %v", cfg.API.DebugMsg)
	log.Println("---------------------------------------------------------------------")
	log.Println("Available Endpoints:")
	log.Println("  GET  /health                  - Health check")
	log.Println("  GET  /v1/models               - List models")
	log.Println("  POST /v1/chat/completions     - OpenAI chat completions")
	log.Println("  POST /v1/messages             - Anthropic messages")
	log.Println("---------------------------------------------------------------------")

	// Start server
	addr := fmt.Sprintf(":%d", cfg.API.Port)
	log.Printf("Server starting on %s", addr)
	log.Println("Press Ctrl+C to stop")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
