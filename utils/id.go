package utils

import (
	"github.com/google/uuid"
)

// GenerateID generates a UUID v4
func GenerateID() string {
	return uuid.New().String()
}

// GenerateChatID generates a chat ID
func GenerateChatID() string {
	return GenerateID()
}

// GenerateRequestID generates a request ID
func GenerateRequestID() string {
	return GenerateID()
}

// GenerateChatCompletionID generates a chat completion ID with prefix
func GenerateChatCompletionID() string {
	return "chatcmpl-" + GenerateID()
}

// GenerateMessageID generates a message ID with prefix
func GenerateMessageID() string {
	return "msg-" + GenerateID()
}