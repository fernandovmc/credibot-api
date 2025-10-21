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
	
	queryParams := map[string]string{
		"select": "*",
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
func GetClientes(c *fiber.Ctx) error {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "25"))

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

	// Get total count first
	countParams := map[string]string{
		"select": "count",
	}
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

	// Fetch limited fields for clientes
	queryParams := map[string]string{
		"select": "id,nome,cpf_cnpj,score_credito,classe_risco,tipo_pessoa,ativo",
		"limit":  strconv.Itoa(perPage),
		"offset": strconv.Itoa(offset),
		"order":  "nome.asc",
	}

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