package handlers

import (
	"context"
	"credibot-api/config"
	"credibot-api/models"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sashabaranov/go-openai"
)

// Chat handles chat requests with OpenAI
func Chat(c *fiber.Ctx) error {
	var req models.ChatRequest
	
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Invalid request format",
			Code:    fiber.StatusBadRequest,
		})
	}

	if req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Message is required",
			Code:    fiber.StatusBadRequest,
		})
	}

	// Default configurations from .env
	if req.Model == "" {
		req.Model = config.AppConfig.OpenAI.Model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = config.AppConfig.OpenAI.MaxTokens
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "OpenAI API key not configured",
			Code:    fiber.StatusInternalServerError,
		})
	}

	client := openai.NewClient(apiKey)
	
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: req.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: req.Message,
				},
			},
			MaxTokens:   req.MaxTokens,
			Temperature: config.AppConfig.OpenAI.Temperature,
		},
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to get response from OpenAI: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	if len(resp.Choices) == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "No response from OpenAI",
			Code:    fiber.StatusInternalServerError,
		})
	}

	response := models.ChatResponse{
		Message: resp.Choices[0].Message.Content,
		Model:   resp.Model,
		Usage: models.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		CreatedAt: time.Now(),
	}

	return c.JSON(models.SuccessResponse{
		Success: true,
		Data:    response,
		Message: "Chat response generated successfully",
	})
}