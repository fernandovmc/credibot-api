package handlers

import (
	"context"
	"credibot-api/config"
	"credibot-api/models"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sashabaranov/go-openai"
)

// SmartChat handles intelligent chat requests with database integration
func SmartChat(c *fiber.Ctx) error {
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

	// First, determine if the question requires database consultation
	needsDatabase, sqlQuery, err := analyzeQuestionAndGenerateSQL(req.Message)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to analyze question: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	var finalResponse string

	if needsDatabase && sqlQuery != "" {
		// Execute the SQL query against Supabase
		queryResult, err := executeSupabaseQuery(sqlQuery)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Error:   true,
				Message: "Failed to execute database query: " + err.Error(),
				Code:    fiber.StatusInternalServerError,
			})
		}

		// Generate final response based on the data
		finalResponse, err = generateResponseWithData(req.Message, queryResult)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Error:   true,
				Message: "Failed to generate response with data: " + err.Error(),
				Code:    fiber.StatusInternalServerError,
			})
		}
	} else {
		// For general questions, use regular OpenAI chat
		finalResponse, err = generateRegularResponse(req.Message)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Error:   true,
				Message: "Failed to generate response: " + err.Error(),
				Code:    fiber.StatusInternalServerError,
			})
		}
	}

	response := models.SmartChatResponse{
		Message:      finalResponse,
		UsedDatabase: needsDatabase,
		SQLQuery:     sqlQuery,
		DatabaseData: nil, // Removido para melhor performance
		CreatedAt:    time.Now(),
	}

	return c.JSON(models.SuccessResponse{
		Success: true,
		Data:    response,
		Message: "Smart chat response generated successfully",
	})
}

// analyzeQuestionAndGenerateSQL determines if a question needs database access and generates SQL
func analyzeQuestionAndGenerateSQL(question string) (bool, string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return false, "", fmt.Errorf("OpenAI API key not configured")
	}

	client := openai.NewClient(apiKey)

	systemPrompt := `Assistente de análise de crédito com SQL.

TABELAS:
- clientes: nome, score_credito, classe_risco, tipo_pessoa, renda_mensal
- analises_credito: decisao, valor_solicitado, valor_aprovado, cliente_id
- operacoes_credito: valor_contratado, status, modalidade, dias_atraso, cliente_id
- historico_pagamentos: status, valor_pago, dias_atraso, operacao_id
- modalidades_credito: nome, categoria, taxa_minima, taxa_maxima
- score_historico: score_atual, score_anterior, cliente_id

REGRAS:
1. Apenas SELECT permitido
2. Sempre usar LIMIT (max 50)
3. Se precisa de dados: responda EXATAMENTE "SQL: [query sem formatação]"
4. Se não precisa: responda "NO_DATABASE_NEEDED"
5. NÃO use markdown, code blocks ou formatação

EXEMPLO: SQL: SELECT nome FROM clientes LIMIT 10

PERGUNTA: ` + question

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: config.AppConfig.OpenAI.Model, // Use model from .env
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: question},
			},
			MaxTokens:   150, // Reduced for SQL generation
			Temperature: 0.1, // Low temperature for consistent SQL generation
		},
	)

	if err != nil {
		return false, "", err
	}

	if len(resp.Choices) == 0 {
		return false, "", fmt.Errorf("no response from OpenAI")
	}

	response := strings.TrimSpace(resp.Choices[0].Message.Content)
	
	if response == "NO_DATABASE_NEEDED" {
		return false, "", nil
	}

	// Extract SQL from response
	sqlQuery := extractSQLFromResponse(response)
	
	if sqlQuery == "" {
		return false, "", nil
	}

	// Validate SQL for security
	if !isValidSelectQuery(sqlQuery) {
		return false, "", fmt.Errorf("invalid or unsafe SQL query generated")
	}

	return true, sqlQuery, nil
}

// extractSQLFromResponse extracts SQL query from OpenAI response
func extractSQLFromResponse(response string) string {
	// Look for "SQL:" prefix
	if strings.HasPrefix(response, "SQL:") {
		sql := strings.TrimSpace(strings.TrimPrefix(response, "SQL:"))
		return cleanSQLFromMarkdown(sql)
	}
	
	// Look for SQL: anywhere in response
	if idx := strings.Index(response, "SQL:"); idx != -1 {
		sql := strings.TrimSpace(response[idx+4:])
		return cleanSQLFromMarkdown(sql)
	}
	
	// Try to find SQL pattern - more flexible regex
	re := regexp.MustCompile(`(?i)SELECT\s+.+?FROM\s+\w+(?:\s+WHERE\s+.+?)?(?:\s+ORDER\s+BY\s+.+?)?(?:\s+LIMIT\s+\d+)?`)
	return re.FindString(response)
}

// cleanSQLFromMarkdown removes markdown formatting from SQL
func cleanSQLFromMarkdown(sql string) string {
	// Remove markdown code blocks
	sql = strings.ReplaceAll(sql, "```sql", "")
	sql = strings.ReplaceAll(sql, "```SQL", "")
	sql = strings.ReplaceAll(sql, "```", "")
	
	// Remove leading/trailing whitespace and semicolons
	sql = strings.TrimSpace(sql)
	sql = strings.TrimSuffix(sql, ";")
	
	// Clean up multiple spaces and newlines
	re := regexp.MustCompile(`\s+`)
	sql = re.ReplaceAllString(sql, " ")
	
	return strings.TrimSpace(sql)
}

// isValidSelectQuery validates that the SQL query is safe
func isValidSelectQuery(query string) bool {
	if query == "" {
		return false
	}
	
	query = strings.ToUpper(strings.TrimSpace(query))
	
	// Must start with SELECT
	if !strings.HasPrefix(query, "SELECT") {
		return false
	}
	
	// Must contain FROM
	if !strings.Contains(query, "FROM") {
		return false
	}
	
	// Forbidden words
	forbidden := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "TRUNCATE", "EXEC", "EXECUTE", "UNION", "--", "/*"}
	for _, word := range forbidden {
		if strings.Contains(query, word) {
			return false
		}
	}
	
	// Basic structure validation
	if !regexp.MustCompile(`SELECT\s+.+\s+FROM\s+\w+`).MatchString(query) {
		return false
	}
	
	return true
}

// executeSupabaseQuery executes the SQL query against Supabase
func executeSupabaseQuery(sqlQuery string) ([]map[string]interface{}, error) {
	baseURL := os.Getenv("SUPABASE_URL")
	apiKey := os.Getenv("SUPABASE_API_KEY")
	
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("supabase credentials not configured")
	}

	// For now, we'll use the REST API with PostgREST syntax
	// Convert basic SQL to PostgREST format
	restQuery := convertSQLToPostgREST(sqlQuery)
	
	responseBody, err := makeSupabaseRequest("GET", restQuery, nil, nil)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// convertSQLToPostgREST converts basic SQL to PostgREST format (simplified)
func convertSQLToPostgREST(sqlQuery string) string {
	// This is a simplified conversion - in production, you'd want a more robust SQL parser
	query := strings.ToLower(strings.TrimSpace(sqlQuery))
	
	// Extract table name from "FROM table_name"
	re := regexp.MustCompile(`from\s+(\w+)`)
	matches := re.FindStringSubmatch(query)
	if len(matches) < 2 {
		return "clientes" // default table
	}
	
	return matches[1]
}

// generateResponseWithData creates a natural language response based on query results
func generateResponseWithData(originalQuestion string, data []map[string]interface{}) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	client := openai.NewClient(apiKey)

	// Limit data to avoid token overflow - take only first 10 records and summarize
	limitedData := data
	if len(data) > 10 {
		limitedData = data[:10]
	}

	// Create a summary instead of full JSON to save tokens
	dataSummary := createDataSummary(limitedData)
	
	systemPrompt := `Você é um assistente especializado em análise de crédito. 

Baseado nos dados fornecidos do banco de dados, responda à pergunta do usuário de forma natural e informativa.

INSTRUÇÕES:
- Use os dados fornecidos para responder
- Seja claro e objetivo  
- Formate números adequadamente (valores monetários em R$, percentuais com %)
- Destaque informações importantes
- Se não houver dados, informe que não foram encontrados registros
- Limite a resposta a no máximo 300 palavras

RESUMO DOS DADOS: ` + dataSummary

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: config.AppConfig.OpenAI.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: originalQuestion},
			},
			MaxTokens:   400,
			Temperature: config.AppConfig.OpenAI.Temperature,
		},
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// createDataSummary creates a concise summary of data to avoid token overflow
func createDataSummary(data []map[string]interface{}) string {
	if len(data) == 0 {
		return "Nenhum dado encontrado."
	}

	summary := fmt.Sprintf("Total de registros: %d\n\n", len(data))
	
	// Show first few records with key information
	for i, record := range data {
		if i >= 5 { // Limit to first 5 records for summary
			summary += fmt.Sprintf("... e mais %d registros\n", len(data)-5)
			break
		}
		
		summary += fmt.Sprintf("Registro %d:\n", i+1)
		
		// Include only important fields to save tokens
		importantFields := []string{"nome", "score_credito", "classe_risco", "valor_solicitado", 
			"valor_aprovado", "decisao", "status", "modalidade", "dias_atraso", "count", "avg", "sum"}
		
		for _, field := range importantFields {
			if value, exists := record[field]; exists {
				summary += fmt.Sprintf("  %s: %v\n", field, value)
			}
		}
		summary += "\n"
	}
	
	return summary
}

// generateRegularResponse generates a regular OpenAI response for general questions
func generateRegularResponse(question string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	client := openai.NewClient(apiKey)

	systemPrompt := `Você é um assistente especializado em análise de crédito e serviços financeiros.
	
Responda perguntas sobre:
- Conceitos de crédito e financiamento
- Análise de risco
- Scores de crédito
- Modalidades de empréstimo
- Educação financeira

Seja profissional, claro e informativo.`

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: config.AppConfig.OpenAI.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: question},
			},
			MaxTokens:   300,
			Temperature: config.AppConfig.OpenAI.Temperature,
		},
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}