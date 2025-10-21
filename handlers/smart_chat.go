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
// @Summary Smart Chat with Database Integration
// @Description Send a question that can automatically query the database and provide intelligent responses
// @Tags Chat
// @Accept json
// @Produce json
// @Param request body models.ChatRequest true "Smart chat request"
// @Success 200 {object} models.SuccessResponse{data=models.SmartChatResponse}
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /smart-chat [post]
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

TABELAS DISPONÍVEIS:
- clientes: id, nome, cpf_cnpj, score_credito, classe_risco, tipo_pessoa (PF/PJ), renda_mensal, faturamento_anual, uf, cidade, ativo
- analises_credito: id, cliente_id, decisao, valor_solicitado, valor_aprovado, taxa_aprovada, data_analise, modalidade_solicitada
- operacoes_credito: id, cliente_id, valor_contratado, valor_saldo, status, modalidade, dias_atraso, taxa_juros_mensal, data_contratacao
- historico_pagamentos: id, operacao_id, status, valor_pago, valor_parcela, dias_atraso, data_vencimento, data_pagamento
- modalidades_credito: id, nome, categoria, taxa_minima, taxa_maxima, prazo_minimo, prazo_maximo, valor_minimo, valor_maximo

OBSERVAÇÕES:
- Tabela score_historico NÃO está populada - NÃO use em queries
- Use apenas SELECT (INSERT/UPDATE/DELETE proibidos)

REGRAS IMPORTANTES:
1. SEMPRE inclua os campos relevantes no SELECT (não apenas o nome)
2. Para perguntas sobre clientes com scores: SELECT nome, score_credito, classe_risco FROM clientes
3. Para rankings: use ORDER BY com o campo apropriado
4. SEMPRE use LIMIT (máximo 50 registros)
5. Para filtros: use WHERE com condições apropriadas
6. Retorne EXATAMENTE no formato: SQL: [query sem formatação, markdown ou code blocks]
7. Se não precisa de dados do banco: retorne "NO_DATABASE_NEEDED"

EXEMPLOS CORRETOS:
- "Clientes com maior score": SQL: SELECT nome, score_credito, classe_risco FROM clientes WHERE ativo = true ORDER BY score_credito DESC LIMIT 10
- "Operações em atraso": SQL: SELECT o.modalidade, o.dias_atraso, o.valor_saldo FROM operacoes_credito o WHERE o.status = 'Vencido' ORDER BY o.dias_atraso DESC LIMIT 20
- "Score acima de 700": SQL: SELECT nome, score_credito, classe_risco, tipo_pessoa FROM clientes WHERE score_credito > 700 AND ativo = true ORDER BY score_credito DESC LIMIT 30

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

	// Convert SQL to PostgREST format
	tableName, queryParams := convertSQLToPostgREST(sqlQuery)

	// Debug logging - remove in production
	fmt.Printf("[DEBUG] SQL Query: %s\n", sqlQuery)
	fmt.Printf("[DEBUG] Table: %s\n", tableName)
	fmt.Printf("[DEBUG] Query Params: %+v\n", queryParams)

	responseBody, err := makeSupabaseRequest("GET", tableName, nil, queryParams)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[DEBUG] Results count: %d\n", len(result))

	return result, nil
}

// convertSQLToPostgREST converts SQL to PostgREST format
func convertSQLToPostgREST(sqlQuery string) (string, map[string]string) {
	query := strings.ToLower(strings.TrimSpace(sqlQuery))
	queryParams := make(map[string]string)

	// Extract table name from "FROM table_name"
	tableRe := regexp.MustCompile(`from\s+(\w+)`)
	tableMatches := tableRe.FindStringSubmatch(query)
	tableName := "clientes" // default
	if len(tableMatches) >= 2 {
		tableName = tableMatches[1]
	}

	// Extract SELECT fields
	selectRe := regexp.MustCompile(`select\s+(.*?)\s+from`)
	selectMatches := selectRe.FindStringSubmatch(query)
	if len(selectMatches) >= 2 {
		fields := strings.TrimSpace(selectMatches[1])
		if fields != "*" {
			// Convert "campo1, campo2" to "campo1,campo2" (remove spaces)
			fields = regexp.MustCompile(`\s*,\s*`).ReplaceAllString(fields, ",")
			queryParams["select"] = fields
		}
	}

	// Extract WHERE conditions
	whereRe := regexp.MustCompile(`where\s+(.*?)(?:\s+order\s+by|\s+limit|\s*$)`)
	whereMatches := whereRe.FindStringSubmatch(query)
	if len(whereMatches) >= 2 {
		whereClause := strings.TrimSpace(whereMatches[1])
		// Convert basic WHERE to PostgREST format
		// Examples: "score_credito > 800" -> "score_credito=gt.800"
		whereClause = convertWhereClause(whereClause)
		if whereClause != "" {
			// For PostgREST, WHERE conditions are added as individual parameters
			// This is a simplified version - will parse common patterns
			parseWhereConditions(whereClause, queryParams)
		}
	}

	// Extract ORDER BY
	orderRe := regexp.MustCompile(`order\s+by\s+([\w_]+)(?:\s+(asc|desc))?`)
	orderMatches := orderRe.FindStringSubmatch(query)
	if len(orderMatches) >= 2 {
		orderField := orderMatches[1]
		orderDir := "asc"
		if len(orderMatches) >= 3 && orderMatches[2] != "" {
			orderDir = orderMatches[2]
		}
		if orderDir == "desc" {
			queryParams["order"] = orderField + ".desc"
		} else {
			queryParams["order"] = orderField + ".asc"
		}
	}

	// Extract LIMIT
	limitRe := regexp.MustCompile(`limit\s+(\d+)`)
	limitMatches := limitRe.FindStringSubmatch(query)
	if len(limitMatches) >= 2 {
		queryParams["limit"] = limitMatches[1]
	}

	return tableName, queryParams
}

// convertWhereClause converts SQL WHERE clause to PostgREST format
func convertWhereClause(whereClause string) string {
	// Remove extra spaces
	whereClause = regexp.MustCompile(`\s+`).ReplaceAllString(whereClause, " ")
	return strings.TrimSpace(whereClause)
}

// parseWhereConditions parses WHERE conditions and adds them to query params
func parseWhereConditions(whereClause string, queryParams map[string]string) {
	// Split by AND (case insensitive)
	// Use regex to split by 'and' or 'AND'
	conditionRegex := regexp.MustCompile(`\s+and\s+`)
	conditions := conditionRegex.Split(whereClause, -1)

	for _, condition := range conditions {
		condition = strings.TrimSpace(condition)
		if condition == "" {
			continue
		}

		// Parse different operators
		// Note: \w+ matches letters, digits, and underscore (includes field names like score_credito, cpf_cnpj)

		// Greater than or equal: field >= value (must be before >)
		if matches := regexp.MustCompile(`([\w_]+)\s*>=\s*(.+)`).FindStringSubmatch(condition); len(matches) >= 3 {
			field := matches[1]
			value := cleanValue(matches[2])
			queryParams[field] = "gte." + value
			continue
		}

		// Greater than: field > value
		if matches := regexp.MustCompile(`([\w_]+)\s*>\s*(.+)`).FindStringSubmatch(condition); len(matches) >= 3 {
			field := matches[1]
			value := cleanValue(matches[2])
			queryParams[field] = "gt." + value
			continue
		}

		// Less than or equal: field <= value (must be before <)
		if matches := regexp.MustCompile(`([\w_]+)\s*<=\s*(.+)`).FindStringSubmatch(condition); len(matches) >= 3 {
			field := matches[1]
			value := cleanValue(matches[2])
			queryParams[field] = "lte." + value
			continue
		}

		// Less than: field < value
		if matches := regexp.MustCompile(`([\w_]+)\s*<\s*(.+)`).FindStringSubmatch(condition); len(matches) >= 3 {
			field := matches[1]
			value := cleanValue(matches[2])
			queryParams[field] = "lt." + value
			continue
		}

		// Equal: field = value
		if matches := regexp.MustCompile(`([\w_]+)\s*=\s*(.+)`).FindStringSubmatch(condition); len(matches) >= 3 {
			field := matches[1]
			value := cleanValue(matches[2])
			queryParams[field] = "eq." + value
			continue
		}

		// LIKE: field LIKE 'value'
		if matches := regexp.MustCompile(`([\w_]+)\s+like\s+['"](.+)['"]`).FindStringSubmatch(condition); len(matches) >= 3 {
			field := matches[1]
			value := matches[2]
			// Convert SQL LIKE to PostgREST like
			queryParams[field] = "like." + value
			continue
		}
	}
}

// cleanValue removes quotes and trims whitespace from SQL values
func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	// Remove single quotes
	value = strings.Trim(value, "'")
	// Remove double quotes
	value = strings.Trim(value, "\"")
	return value
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