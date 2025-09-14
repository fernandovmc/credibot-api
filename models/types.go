package models

import "time"

// ChatRequest represents a chat request to OpenAI
type ChatRequest struct {
	Message   string `json:"message" validate:"required,min=1"`
	Model     string `json:"model,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// ChatResponse represents the chat response
type ChatResponse struct {
	Message   string    `json:"message"`
	Model     string    `json:"model"`
	Usage     Usage     `json:"usage"`
	CreatedAt time.Time `json:"created_at"`
}

// SmartChatResponse represents the smart chat response with database integration
type SmartChatResponse struct {
	Message      string      `json:"message"`
	UsedDatabase bool        `json:"used_database"`
	SQLQuery     string      `json:"sql_query,omitempty"`
	DatabaseData interface{} `json:"database_data,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}

// Usage represents OpenAI API usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// DatabaseRecord represents a generic Supabase record
type DatabaseRecord struct {
	ID        string                 `json:"id,omitempty"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at,omitempty"`
	UpdatedAt time.Time              `json:"updated_at,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// SupabaseConfig contains Supabase configurations
type SupabaseConfig struct {
	URL    string
	APIKey string
}

// OpenAIConfig contains OpenAI configurations
type OpenAIConfig struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float32
}