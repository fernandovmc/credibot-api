package handlers

import (
	"bytes"
	"credibot-api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// makeSupabaseRequest makes HTTP requests to Supabase REST API
func makeSupabaseRequest(method, table string, body interface{}, queryParams map[string]string) ([]byte, error) {
	baseURL := os.Getenv("SUPABASE_URL")
	apiKey := os.Getenv("SUPABASE_API_KEY")
	
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("supabase credentials not configured")
	}

	urlStr := fmt.Sprintf("%s/rest/v1/%s", baseURL, table)

	// Add query parameters with proper URL encoding
	if len(queryParams) > 0 {
		queryString := ""
		first := true
		for key, value := range queryParams {
			if !first {
				queryString += "&"
			}
			// Properly encode both key and value
			queryString += fmt.Sprintf("%s=%s", url.QueryEscape(key), url.QueryEscape(value))
			first = false
		}
		if queryString != "" {
			urlStr += "?" + queryString
		}
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, urlStr, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", apiKey)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("supabase error: %s", string(responseBody))
	}

	return responseBody, nil
}

// GetData fetches data from a specific table
// @Summary Get data from any table
// @Description Generic endpoint to fetch data from any database table
// @Tags Database
// @Accept json
// @Produce json
// @Param table path string true "Table name"
// @Param limit query int false "Limit" default(10)
// @Param offset query int false "Offset" default(0)
// @Param order_by query string false "Order by field" default(created_at)
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /data/{table} [get]
func GetData(c *fiber.Ctx) error {
	table := c.Params("table")
	if table == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Table name is required",
			Code:    fiber.StatusBadRequest,
		})
	}

	// Optional query parameters with safety limits
	limit := c.Query("limit", "10")
	offset := c.Query("offset", "0")
	orderBy := c.Query("order_by", "created_at")

	limitInt, _ := strconv.Atoi(limit)
	if limitInt > 50 { // Safety limit to prevent token overflow
		limitInt = 50
	}

	// Define select fields based on table
	selectFields := "*"
	if table == "clientes" {
		// For clientes table, return only essential fields
		selectFields = "id,nome,cpf_cnpj,score_credito,classe_risco,tipo_pessoa,ativo"
	}

	queryParams := map[string]string{
		"select": selectFields,
		"limit":  strconv.Itoa(limitInt), // Use the safety-limited value
		"offset": offset,
		"order":  orderBy + ".desc",
	}

	responseBody, err := makeSupabaseRequest("GET", table, nil, queryParams)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to fetch data: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	var data []map[string]interface{}
	err = json.Unmarshal(responseBody, &data)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to parse response: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	return c.JSON(models.SuccessResponse{
		Success: true,
		Data:    data,
		Message: "Data retrieved successfully",
	})
}

// InsertData inserts new data into a table
func InsertData(c *fiber.Ctx) error {
	table := c.Params("table")
	if table == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Table name is required",
			Code:    fiber.StatusBadRequest,
		})
	}

	var data map[string]interface{}
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Invalid request format",
			Code:    fiber.StatusBadRequest,
		})
	}

	responseBody, err := makeSupabaseRequest("POST", table, data, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to insert data: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	var result []map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to parse response: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse{
		Success: true,
		Data:    result,
		Message: "Data inserted successfully",
	})
}

// UpdateData updates existing data
func UpdateData(c *fiber.Ctx) error {
	table := c.Params("table")
	id := c.Params("id")
	
	if table == "" || id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Table name and ID are required",
			Code:    fiber.StatusBadRequest,
		})
	}

	var data map[string]interface{}
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Invalid request format",
			Code:    fiber.StatusBadRequest,
		})
	}

	queryParams := map[string]string{
		"id": "eq." + id,
	}

	responseBody, err := makeSupabaseRequest("PATCH", table, data, queryParams)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to update data: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	var result []map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to parse response: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	return c.JSON(models.SuccessResponse{
		Success: true,
		Data:    result,
		Message: "Data updated successfully",
	})
}

// DeleteData deletes existing data
func DeleteData(c *fiber.Ctx) error {
	table := c.Params("table")
	id := c.Params("id")
	
	if table == "" || id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Table name and ID are required",
			Code:    fiber.StatusBadRequest,
		})
	}

	queryParams := map[string]string{
		"id": "eq." + id,
	}

	responseBody, err := makeSupabaseRequest("DELETE", table, nil, queryParams)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to delete data: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	var result []map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to parse response: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	return c.JSON(models.SuccessResponse{
		Success: true,
		Data:    result,
		Message: "Data deleted successfully",
	})
}

// buildSupabaseFilterQuery builds a Supabase query string with proper filter syntax
// Returns a query string that can have multiple filters for the same field
func buildSupabaseFilterQuery(filters map[string][]string, pagination map[string]string) string {
	params := url.Values{}

	// Add pagination and select parameters
	for key, value := range pagination {
		params.Add(key, value)
	}

	// Add filters - note that PostgREST allows multiple filters on same field with AND logic
	for field, values := range filters {
		for _, value := range values {
			if value != "" {
				params.Add(field, value)
			}
		}
	}

	return params.Encode()
}

// GetClientes fetches clientes with pagination and filters
// @Summary List clientes with pagination and filters
// @Description Get a paginated list of clientes with essential fields and optional filters
// @Tags Clientes
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(25)
// @Param search query string false "Search by name (case-insensitive partial match)"
// @Param score_min query int false "Minimum credit score (0-1000)"
// @Param score_max query int false "Maximum credit score (0-1000)"
// @Param classe_risco query string false "Filter by risk class"
// @Param tipo_pessoa query string false "Filter by person type (PF or PJ)"
// @Param ativo query boolean false "Filter by active status"
// @Success 200 {object} models.PaginatedResponse{data=[]models.ClienteSummary}
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /clientes [get]
func GetClientes(c *fiber.Ctx) error {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "25"))

	// Parse filter parameters
	search := c.Query("search")
	scoreMinStr := c.Query("score_min")
	scoreMaxStr := c.Query("score_max")
	classeRisco := c.Query("classe_risco")
	tipoPessoa := c.Query("tipo_pessoa")
	ativoStr := c.Query("ativo")

	// Validate and limit pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 25
	}
	if perPage > 100 {
		perPage = 100 // Maximum limit for safety
	}

	// Validate filter parameters
	var scoreMin, scoreMax int
	if scoreMinStr != "" {
		var err error
		scoreMin, err = strconv.Atoi(scoreMinStr)
		if err != nil || scoreMin < 0 || scoreMin > 1000 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Error:   true,
				Message: "score_min must be a number between 0 and 1000",
				Code:    fiber.StatusBadRequest,
			})
		}
	}

	if scoreMaxStr != "" {
		var err error
		scoreMax, err = strconv.Atoi(scoreMaxStr)
		if err != nil || scoreMax < 0 || scoreMax > 1000 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Error:   true,
				Message: "score_max must be a number between 0 and 1000",
				Code:    fiber.StatusBadRequest,
			})
		}
	}

	// Validate that score_min <= score_max
	if scoreMinStr != "" && scoreMaxStr != "" && scoreMin > scoreMax {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "score_min must be less than or equal to score_max",
			Code:    fiber.StatusBadRequest,
		})
	}

	// Validate tipo_pessoa
	if tipoPessoa != "" && tipoPessoa != "PF" && tipoPessoa != "PJ" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "tipo_pessoa must be 'PF' or 'PJ'",
			Code:    fiber.StatusBadRequest,
		})
	}

	// Validate ativo (should be true or false)
	if ativoStr != "" && ativoStr != "true" && ativoStr != "false" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "ativo must be 'true' or 'false'",
			Code:    fiber.StatusBadRequest,
		})
	}

	// Calculate offset for pagination
	offset := (page - 1) * perPage

	// Build filter parameters using URL values to support multiple filters on same field
	// Count query filters
	countFilters := make(map[string][]string)
	if search != "" {
		countFilters["nome"] = []string{"ilike.*" + search + "*"}
	}
	if scoreMinStr != "" {
		countFilters["score_credito"] = []string{"gte." + scoreMinStr}
	}
	if scoreMaxStr != "" {
		// For score_max, we add it as an additional filter on the same field
		if scoreMinStr != "" {
			countFilters["score_credito"] = append(countFilters["score_credito"], "lte."+scoreMaxStr)
		} else {
			countFilters["score_credito"] = []string{"lte." + scoreMaxStr}
		}
	}
	if classeRisco != "" {
		countFilters["classe_risco"] = []string{"eq." + classeRisco}
	}
	if tipoPessoa != "" {
		countFilters["tipo_pessoa"] = []string{"eq." + tipoPessoa}
	}
	if ativoStr != "" {
		countFilters["ativo"] = []string{"eq." + ativoStr}
	}

	// Count pagination parameters
	countPagination := map[string]string{
		"select": "count",
	}

	// Build count query string using url.Values for proper encoding
	countQueryString := buildSupabaseFilterQuery(countFilters, countPagination)

	// Get total count with filters using custom URL construction
	baseURL := os.Getenv("SUPABASE_URL")
	apiKey := os.Getenv("SUPABASE_API_KEY")

	if baseURL == "" || apiKey == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Supabase credentials not configured",
			Code:    fiber.StatusInternalServerError,
		})
	}

	countURL := fmt.Sprintf("%s/rest/v1/clientes?%s", baseURL, countQueryString)
	countReq, _ := http.NewRequest("GET", countURL, nil)
	countReq.Header.Set("apikey", apiKey)
	countReq.Header.Set("Authorization", "Bearer "+apiKey)
	countReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	countResp, err := client.Do(countReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to get total count: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}
	defer countResp.Body.Close()

	countBody, _ := io.ReadAll(countResp.Body)

	var countResult []map[string]interface{}
	json.Unmarshal(countBody, &countResult)

	total := 0
	if len(countResult) > 0 {
		if count, ok := countResult[0]["count"].(float64); ok {
			total = int(count)
		}
	}

	// Build data query filters (same as count but with pagination)
	dataFilters := make(map[string][]string)
	if search != "" {
		dataFilters["nome"] = []string{"ilike.*" + search + "*"}
	}
	if scoreMinStr != "" {
		dataFilters["score_credito"] = []string{"gte." + scoreMinStr}
	}
	if scoreMaxStr != "" {
		if scoreMinStr != "" {
			dataFilters["score_credito"] = append(dataFilters["score_credito"], "lte."+scoreMaxStr)
		} else {
			dataFilters["score_credito"] = []string{"lte." + scoreMaxStr}
		}
	}
	if classeRisco != "" {
		dataFilters["classe_risco"] = []string{"eq." + classeRisco}
	}
	if tipoPessoa != "" {
		dataFilters["tipo_pessoa"] = []string{"eq." + tipoPessoa}
	}
	if ativoStr != "" {
		dataFilters["ativo"] = []string{"eq." + ativoStr}
	}

	// Data pagination parameters
	dataPagination := map[string]string{
		"select": "id,nome,cpf_cnpj,score_credito,classe_risco,tipo_pessoa,ativo",
		"limit":  strconv.Itoa(perPage),
		"offset": strconv.Itoa(offset),
		"order":  "nome.asc",
	}

	dataQueryString := buildSupabaseFilterQuery(dataFilters, dataPagination)
	dataURL := fmt.Sprintf("%s/rest/v1/clientes?%s", baseURL, dataQueryString)

	dataReq, _ := http.NewRequest("GET", dataURL, nil)
	dataReq.Header.Set("apikey", apiKey)
	dataReq.Header.Set("Authorization", "Bearer "+apiKey)
	dataReq.Header.Set("Content-Type", "application/json")

	dataResp, err := client.Do(dataReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to fetch clientes: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}
	defer dataResp.Body.Close()

	responseBody, _ := io.ReadAll(dataResp.Body)

	var clientes []models.ClienteSummary
	if err := json.Unmarshal(responseBody, &clientes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to parse clientes: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	// Calculate total pages
	totalPages := (total + perPage - 1) / perPage

	return c.JSON(models.PaginatedResponse{
		Success: true,
		Data:    clientes,
		Pagination: models.Pagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
		Message: "Clientes retrieved successfully",
	})
}

// GetClienteByID fetches a single cliente with all fields by ID
// @Summary Get cliente by ID
// @Description Fetch a single cliente with all data by UUID
// @Tags Clientes
// @Accept json
// @Produce json
// @Param id path string true "Cliente UUID"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /cliente/{id} [get]
func GetClienteByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Cliente ID is required",
			Code:    fiber.StatusBadRequest,
		})
	}

	queryParams := map[string]string{
		"id": "eq." + id,
	}

	responseBody, err := makeSupabaseRequest("GET", "clientes", nil, queryParams)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to fetch cliente: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	var clientes []map[string]interface{}
	if err := json.Unmarshal(responseBody, &clientes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to parse cliente: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	if len(clientes) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Cliente not found",
			Code:    fiber.StatusNotFound,
		})
	}

	return c.JSON(models.SuccessResponse{
		Success: true,
		Data:    clientes[0],
		Message: "Cliente retrieved successfully",
	})
}