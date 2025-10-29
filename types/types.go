package types

// Message represents a chat message
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // Can be string or []ContentBlock
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentBlock represents a content block (text, image, tool use, etc.)
type ContentBlock struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	ImageURL *ImageURL              `json:"image_url,omitempty"`
	Source   *ImageSource           `json:"source,omitempty"`
	ID       string                 `json:"id,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
}

// ImageURL represents an image URL (OpenAI format)
type ImageURL struct {
	URL string `json:"url"`
}

// ImageSource represents an image source (Anthropic format)
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// ToolCall represents a tool call
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function *FunctionCall          `json:"function,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model            string                 `json:"model"`
	Messages         []Message              `json:"messages"`
	Stream           bool                   `json:"stream"`
	Temperature      float64                `json:"temperature,omitempty"`
	MaxTokens        int                    `json:"max_tokens,omitempty"`
	TopP             float64                `json:"top_p,omitempty"`
	System           interface{}            `json:"system,omitempty"`
	Thinking         map[string]interface{} `json:"thinking,omitempty"`
	EnableThinking   interface{}            `json:"enable_thinking,omitempty"`
	StreamOptions    *StreamOptions         `json:"stream_options,omitempty"`
	Tools            []Tool                 `json:"tools,omitempty"`
	FrequencyPenalty float64                `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64                `json:"presence_penalty,omitempty"`
}

// StreamOptions represents streaming options
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// Tool represents a tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents a response choice
type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`
	Delta        *Message `json:"delta,omitempty"`
	FinishReason *string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Model represents a model
type Model struct {
	ID           string                 `json:"id"`
	Object       string                 `json:"object"`
	Name         string                 `json:"name"`
	Meta         map[string]interface{} `json:"meta"`
	Info         map[string]interface{} `json:"info"`
	Created      int64                  `json:"created"`
	OwnedBy      string                 `json:"owned_by"`
	Original     map[string]interface{} `json:"orignal"` // Keep typo for compatibility
	AccessControl interface{}           `json:"access_control"`
}

// ModelsResponse represents a models list response
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// UserInfo represents user information
type UserInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// ZaiRequest represents a request to Z.ai API
type ZaiRequest struct {
	Model            string                 `json:"model"`
	Messages         []Message              `json:"messages"`
	Stream           bool                   `json:"stream"`
	ChatID           string                 `json:"chat_id"`
	ID               string                 `json:"id"`
	Features         map[string]interface{} `json:"features,omitempty"`
	SignaturePrompt  string                 `json:"signature_prompt,omitempty"`
	Temperature      float64                `json:"temperature,omitempty"`
	MaxTokens        int                    `json:"max_tokens,omitempty"`
	TopP             float64                `json:"top_p,omitempty"`
	FrequencyPenalty float64                `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64                `json:"presence_penalty,omitempty"`
}

// ZaiResponse represents a response from Z.ai API
type ZaiResponse struct {
	Data *ZaiResponseData `json:"data"`
}

// ZaiResponseData represents the data field in Z.ai response
type ZaiResponseData struct {
	Phase        string `json:"phase"`
	DeltaContent string `json:"delta_content"`
	EditContent  string `json:"edit_content"`
	Done         bool   `json:"done"`
}

// AnthropicMessageRequest represents an Anthropic messages request
type AnthropicMessageRequest struct {
	Model     string                 `json:"model"`
	MaxTokens int                    `json:"max_tokens"`
	Messages  []Message              `json:"messages"`
	System    interface{}            `json:"system,omitempty"`
	Stream    bool                   `json:"stream,omitempty"`
	Tools     []Tool                 `json:"tools,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AnthropicMessageResponse represents an Anthropic messages response
type AnthropicMessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	Content      []ContentBlock `json:"content"`
	Usage        *Usage         `json:"usage,omitempty"`
	StopSequence *string        `json:"stop_sequence"`
	StopReason   string         `json:"stop_reason"`
}

// AnthropicStreamEvent represents an Anthropic streaming event
type AnthropicStreamEvent struct {
	Type         string                 `json:"type"`
	Message      *AnthropicMessage      `json:"message,omitempty"`
	Index        int                    `json:"index,omitempty"`
	ContentBlock *ContentBlock          `json:"content_block,omitempty"`
	Delta        map[string]interface{} `json:"delta,omitempty"`
	Usage        *Usage                 `json:"usage,omitempty"`
}

// AnthropicMessage represents an Anthropic message in streaming
type AnthropicMessage struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Role         string  `json:"role"`
	Model        string  `json:"model"`
	StopReason   *string `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
	Usage        *Usage  `json:"usage"`
}

// ImageUploadResponse represents an image upload response
type ImageUploadResponse struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}