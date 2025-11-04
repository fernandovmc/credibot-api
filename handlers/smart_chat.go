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
2. Para perguntas sobre clientes com scores: SELECT nome, score_credito, classe_risco, tipo_pessoa FROM clientes
3. Para rankings: OBRIGATORIAMENTE use ORDER BY com o campo apropriado
4. LIMITE CRITICAL - USE ISTO PARA CADA TIPO DE PERGUNTA:
   *** Para "clientes com menor/maior score": SEMPRE usar LIMIT 100 (NÃO USAR LIMIT 10!) ***
   *** Para "top 10/20": SEMPRE usar LIMIT conforme pedido (ex: LIMIT 20 para "top 20") ***
   *** Para "análises" ou "distribuição": usar LIMIT 100 ***
   *** MÁXIMO PERMITIDO: 100 registros ***
   NÃO VIOLE ISTO - O USUÁRIO QUER DADOS DETALHADOS!
5. Para filtros: use WHERE com condições apropriadas
6. Retorne EXATAMENTE no formato: SQL: [query sem formatação, markdown ou code blocks]
7. Se não precisa de dados do banco: retorne "NO_DATABASE_NEEDED"

EXEMPLOS CORRETOS E OBRIGATÓRIOS:
❌ ERRADO: SELECT nome, score_credito FROM clientes ORDER BY score_credito ASC LIMIT 10
✅ CORRETO: SELECT nome, score_credito, classe_risco, tipo_pessoa FROM clientes ORDER BY score_credito ASC LIMIT 100

❌ ERRADO: SELECT nome, score_credito FROM clientes ORDER BY score_credito DESC LIMIT 10
✅ CORRETO: SELECT nome, score_credito, classe_risco, tipo_pessoa FROM clientes ORDER BY score_credito DESC LIMIT 100

Se perguntam "clientes com menor score" = SEMPRE LIMIT 100
Se perguntam "clientes com maior score" = SEMPRE LIMIT 100
Se perguntam "clientes" em geral = SEMPRE LIMIT 100
Se perguntam "top 10" ou "10 melhores" = LIMIT 10
Se perguntam "top 20" ou "20 melhores" = LIMIT 20

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
			Temperature: 0.5, // Increased for better instruction following (not too low, not too high)
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

	// Limit data intelligently to balance detail and token usage
	// For small datasets (<50): pass all
	// For medium datasets (50-200): pass first 100 with stats
	// For large datasets (>200): pass first 200 with comprehensive stats
	limitedData := data
	maxRecordsToSummary := 100
	if len(data) > 200 {
		maxRecordsToSummary = 200
		limitedData = data[:maxRecordsToSummary]
	} else if len(data) > 100 {
		maxRecordsToSummary = 100
		limitedData = data[:maxRecordsToSummary]
	}

	// Create a detailed summary with statistics instead of full JSON to save tokens
	dataSummary := createDetailedDataSummary(limitedData, len(data))
	
	systemPrompt := `Você é um assistente especializado em análise de crédito e risco bancário.

Baseado nos dados fornecidos do banco de dados, responda à pergunta do usuário de forma DETALHADA, PROFISSIONAL e bem FORMATADA.

TIPOS DE PERGUNTAS QUE VOCÊ RESPONDE:
1. "Qual é o histórico de score do cliente CPF xxx" → Retorne histórico de scores com datas
2. "Faça uma análise de crédito do cliente ID xxx" → Análise completa: scores, operações, pagamentos, risco
3. "Faça uma análise de crédito do cliente João Gomes" → Busque por nome e faça análise completa
4. "Faça uma análise de crédito do cliente CPF 321.321" → Busque por CPF e faça análise completa
5. "Devo aprovar crédito pro cliente x? Justifique" → Recomendação fundada em dados com riscos e benefícios
6. "Quais clientes têm menor/maior score" → Ranking com estatísticas e distribuição
7. "Quais operações estão em atraso" → Lista de operações vencidas com análise
8. "Qual é a distribuição de clientes por classe de risco" → Tabela de distribuição em %

INSTRUÇÕES OBRIGATÓRIAS DE FORMATAÇÃO:
- Use MARKDOWN para estruturar respostas
- Use **negrito** para destacar números importantes
- Use ### para subtítulos de seções
- Use tabelas Markdown para comparações e dados estruturados
- Use listas numeradas (1., 2., 3.) para recomendações
- Use ✅/❌ para indicar aprovação/rejeição em análises
- Use emojis relevantes para melhor legibilidade (📊, 📈, ⚠️, ✓, ✗, 💰, 🏦, 📋)

EXEMPLOS DE FORMATAÇÃO:
### Análise de Crédito
**Cliente:** João Silva | **CPF:** 123.456.789-00 | **Score:** 750

**Indicadores Principais:**
| Métrica | Valor | Status |
|---------|-------|--------|
| Score de Crédito | 750 | ✅ Bom |
| Classe de Risco | A | ✅ Baixo |
| Operações em Atraso | 0 | ✅ Nenhuma |

**Recomendação:** ✅ **APROVADO** - Cliente apresenta bom score e histórico

INSTRUÇÕES DE CONTEÚDO:
1. Use SEMPRE os dados fornecidos para responder - cite estatísticas, mínimos, máximos, médias
2. Seja claro, objetivo e MUITO DETALHADO
3. Formate números adequadamente (valores monetários em R$, percentuais com %)
4. Destaque informações importantes e insights com negrito/emojis
5. Se a pergunta pede "clientes com menor/maior score": SEMPRE cite:
   - O score mínimo e máximo encontrado
   - Quantos clientes em cada faixa de score
   - A distribuição por classe de risco em tabela
   - Exemplos concretos de clientes com scores extremos
6. Mostre a VARIAÇÃO e DISTRIBUIÇÃO dos dados, NÃO apenas um único valor
7. Em análises de crédito, sempre forneça:
   - Resumo dos dados do cliente
   - Tabela de indicadores principais
   - Análise de risco detalhada
   - Recomendação fundamentada (Aprovado/Reprovado/Condicional)
8. Se não houver dados, informe que não foram encontrados registros
9. Limite a resposta a no máximo 800 palavras
10. Sempre termine análises de crédito com recomendação clara (✅/❌/⚠️)

RESUMO DOS DADOS FORNECIDOS: ` + dataSummary

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: config.AppConfig.OpenAI.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: originalQuestion},
			},
			MaxTokens:   1000, // Increased for detailed responses with markdown formatting and tables
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

// createDetailedDataSummary creates a comprehensive summary with statistics
// totalCount includes records not shown in data slice
func createDetailedDataSummary(data []map[string]interface{}, totalCount int) string {
	if len(data) == 0 {
		return "Nenhum dado encontrado."
	}

	summary := fmt.Sprintf("=== RESUMO DOS DADOS ===\n")
	summary += fmt.Sprintf("Total de registros retornados: %d\n", len(data))
	if totalCount > len(data) {
		summary += fmt.Sprintf("Total de registros no banco (com filtros): %d\n", totalCount)
	} else {
		summary += fmt.Sprintf("Total de registros no banco (com filtros): %d\n", totalCount)
	}
	summary += "\n"

	// Calculate statistics for numeric fields
	stats := calculateStatistics(data)
	if len(stats) > 0 {
		summary += "=== ESTATÍSTICAS ===\n"
		for field, fieldStats := range stats {
			summary += fmt.Sprintf("\n%s:\n", field)
			summary += fmt.Sprintf("  - Mínimo: %v\n", fieldStats["min"])
			summary += fmt.Sprintf("  - Máximo: %v\n", fieldStats["max"])
			summary += fmt.Sprintf("  - Média: %.2f\n", fieldStats["avg"].(float64))
			summary += fmt.Sprintf("  - Registros com valor: %d\n", fieldStats["count"])
		}
		summary += "\n"
	}

	// Show distribution for categorical fields
	distributions := calculateDistribution(data)
	if len(distributions) > 0 {
		summary += "=== DISTRIBUIÇÃO POR CATEGORIA ===\n"
		for field, dist := range distributions {
			summary += fmt.Sprintf("\n%s:\n", field)
			for value, count := range dist {
				percentage := float64(count) / float64(len(data)) * 100
				summary += fmt.Sprintf("  - %v: %d registros (%.1f%%)\n", value, count, percentage)
			}
		}
		summary += "\n"
	}

	// Show actual records with detailed information
	summary += "=== DETALHES DOS REGISTROS ===\n"
	recordsToShow := 20 // Show more records for better analysis
	if len(data) < recordsToShow {
		recordsToShow = len(data)
	}

	for i := 0; i < recordsToShow; i++ {
		record := data[i]
		summary += fmt.Sprintf("\nRegistro %d:\n", i+1)

		// Show important fields in priority order
		importantFields := []string{"nome", "score_credito", "classe_risco", "tipo_pessoa",
			"valor_solicitado", "valor_aprovado", "decisao", "status", "modalidade",
			"dias_atraso", "taxa_aprovada", "renda_mensal", "uf", "cidade", "ativo"}

		for _, field := range importantFields {
			if value, exists := record[field]; exists {
				summary += fmt.Sprintf("  %s: %v\n", field, value)
			}
		}
	}

	if recordsToShow < len(data) {
		summary += fmt.Sprintf("\n... e mais %d registros (mostrando detalhes dos primeiros %d)\n",
			len(data)-recordsToShow, recordsToShow)
	}

	return summary
}

// calculateStatistics computes min, max, avg, count for numeric fields
func calculateStatistics(data []map[string]interface{}) map[string]map[string]interface{} {
	stats := make(map[string]map[string]interface{})
	numericFields := []string{"score_credito", "valor_solicitado", "valor_aprovado",
		"taxa_aprovada", "renda_mensal", "faturamento_anual", "dias_atraso",
		"taxa_juros_mensal", "valor_contratado", "valor_saldo"}

	for _, field := range numericFields {
		var values []float64
		for _, record := range data {
			if val, exists := record[field]; exists && val != nil {
				switch v := val.(type) {
				case float64:
					values = append(values, v)
				case int:
					values = append(values, float64(v))
				}
			}
		}

		if len(values) > 0 {
			min := values[0]
			max := values[0]
			sum := 0.0

			for _, v := range values {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
				sum += v
			}

			avg := sum / float64(len(values))
			stats[field] = map[string]interface{}{
				"min":   min,
				"max":   max,
				"avg":   avg,
				"count": len(values),
			}
		}
	}

	return stats
}

// calculateDistribution counts occurrences of categorical field values
func calculateDistribution(data []map[string]interface{}) map[string]map[interface{}]int {
	dist := make(map[string]map[interface{}]int)
	categoricalFields := []string{"classe_risco", "tipo_pessoa", "decisao", "status",
		"modalidade", "ativo", "uf"}

	for _, field := range categoricalFields {
		fieldDist := make(map[interface{}]int)
		for _, record := range data {
			if val, exists := record[field]; exists && val != nil {
				fieldDist[val]++
			}
		}
		if len(fieldDist) > 0 {
			dist[field] = fieldDist
		}
	}

	return dist
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