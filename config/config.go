package config

import (
	"credibot-api/models"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config contains all application configurations
type Config struct {
	Port     string
	Supabase models.SupabaseConfig
	OpenAI   models.OpenAIConfig
}

var AppConfig *Config

// LoadConfig loads configurations from environment
func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found, using system environment variables")
	}

	AppConfig = &Config{
		Port: getEnv("PORT", "3000"),
		Supabase: models.SupabaseConfig{
			URL:    getEnv("SUPABASE_URL", ""),
			APIKey: getEnv("SUPABASE_API_KEY", ""),
		},
		OpenAI: models.OpenAIConfig{
			APIKey:      getEnv("OPENAI_API_KEY", ""),
			Model:       getEnv("OPENAI_MODEL", "gpt-3.5-turbo"),
			MaxTokens:   getEnvAsInt("OPENAI_MAX_TOKENS", 150),
			Temperature: getEnvAsFloat("OPENAI_TEMPERATURE", 0.7),
		},
	}

	validateConfig()
}

// getEnv gets an environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as int with default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsFloat gets an environment variable as float32 with default value
func getEnvAsFloat(key string, defaultValue float32) float32 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 32); err == nil {
			return float32(floatValue)
		}
	}
	return defaultValue
}

// validateConfig validates required configurations
func validateConfig() {
	warnings := []string{}

	if AppConfig.Supabase.URL == "" {
		warnings = append(warnings, "SUPABASE_URL not configured")
	}
	if AppConfig.Supabase.APIKey == "" {
		warnings = append(warnings, "SUPABASE_API_KEY not configured")
	}
	if AppConfig.OpenAI.APIKey == "" {
		warnings = append(warnings, "OPENAI_API_KEY not configured")
	}

	if len(warnings) > 0 {
		log.Println("Configuration warnings:")
		for _, warning := range warnings {
			log.Printf("   - %s", warning)
		}
	} else {
		log.Println("All configurations loaded successfully")
	}
}