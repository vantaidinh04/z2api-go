package utils

import (
	"log"
	"os"
	"path/filepath"

	"github.com/pkoukk/tiktoken-go"
)

var encoder *tiktoken.Tiktoken

// InitTokenizer initializes the tiktoken encoder
func InitTokenizer() error {
	// Set cache directory
	cacheDir := filepath.Join(".", "tiktoken")
	os.Setenv("TIKTOKEN_CACHE_DIR", cacheDir)

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	// Initialize encoder
	var err error
	encoder, err = tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return err
	}

	log.Println("Tokenizer initialized successfully")
	return nil
}

// CountTokens counts tokens in text using tiktoken
func CountTokens(text string) int {
	if encoder == nil {
		if err := InitTokenizer(); err != nil {
			log.Printf("Warning: Failed to initialize tokenizer: %v", err)
			return 0
		}
	}

	tokens := encoder.Encode(text, nil, nil)
	return len(tokens)
}