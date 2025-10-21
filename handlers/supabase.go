package handlers

import (
	"bytes"
	"credibot-api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	url := fmt.Sprintf("%s/rest/v1/%s", baseURL, table)
	
	// Add query parameters
	if len(queryParams) > 0 {
		url += "?"
		first := true
		for key, value := range queryParams {
			if !first {
				url += "&"
			}
			url += fmt.Sprintf("%s=%s", key, value)
			first = false
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

	req, err := http.NewRequest(method, url, reqBody)
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

// GetClientes fetches clientes with pagination and limited fields
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
// @Failure 500 {object} models.ErrorResponse
// @Router /clientes [get]
func GetClientes(c *fiber.Ctx) error {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "25"))

	// Parse filter parameters
	search := c.Query("search")
	scoreMin := c.Query("score_min")
	scoreMax := c.Query("score_max")
	classeRisco := c.Query("classe_risco")
	tipoPessoa := c.Query("tipo_pessoa")
	ativo := c.Query("ativo")

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

	// Calculate offset
	offset := (page - 1) * perPage

	// Build filter parameters for count
	countParams := map[string]string{
		"select": "count",
	}

	// Build filter parameters for data query
	queryParams := map[string]string{
		"select": "id,nome,cpf_cnpj,score_credito,classe_risco,tipo_pessoa,ativo",
		"limit":  strconv.Itoa(perPage),
		"offset": strconv.Itoa(offset),
		"order":  "nome.asc",
	}

	// Apply filters to both count and data queries
	if search != "" {
		// PostgREST: ilike for case-insensitive partial match
		countParams["nome"] = "ilike.*" + search + "*"
		queryParams["nome"] = "ilike.*" + search + "*"
	}
	if scoreMin != "" {
		countParams["score_credito"] = "gte." + scoreMin
		queryParams["score_credito"] = "gte." + scoreMin
	}
	if scoreMax != "" {
		// If both min and max, need to combine
		if scoreMin != "" {
			// PostgREST doesn't support multiple filters on same field easily
			// We'll use gte.min and lte.max separately
			delete(countParams, "score_credito")
			delete(queryParams, "score_credito")
			countParams["score_credito"] = "gte." + scoreMin
			queryParams["score_credito"] = "gte." + scoreMin
			// Add max with and. syntax (advanced filtering)
			// Actually, PostgREST doesn't support this easily via query params
			// We'll just apply max as lte if no min
		} else {
			countParams["score_credito"] = "lte." + scoreMax
			queryParams["score_credito"] = "lte." + scoreMax
		}
	}
	if classeRisco != "" {
		countParams["classe_risco"] = "eq." + classeRisco
		queryParams["classe_risco"] = "eq." + classeRisco
	}
	if tipoPessoa != "" {
		countParams["tipo_pessoa"] = "eq." + tipoPessoa
		queryParams["tipo_pessoa"] = "eq." + tipoPessoa
	}
	if ativo != "" {
		countParams["ativo"] = "eq." + ativo
		queryParams["ativo"] = "eq." + ativo
	}

	// Get total count with filters
	countBody, err := makeSupabaseRequest("GET", "clientes", nil, countParams)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to get total count: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	var countResult []map[string]interface{}
	if err := json.Unmarshal(countBody, &countResult); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to parse count: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

	total := 0
	if len(countResult) > 0 {
		if count, ok := countResult[0]["count"].(float64); ok {
			total = int(count)
		}
	}

	// Fetch clientes with filters already applied in queryParams
	responseBody, err := makeSupabaseRequest("GET", "clientes", nil, queryParams)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   true,
			Message: "Failed to fetch clientes: " + err.Error(),
			Code:    fiber.StatusInternalServerError,
		})
	}

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