package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"tokenpagination/repository"
)

// MockRecordRepository is a mock implementation of RecordRepositoryInterface for testing
type MockRecordRepository struct {
	mock.Mock
}

func (m *MockRecordRepository) CreateTable() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRecordRepository) Insert(resourceID, resourceType string, context *string) error {
	args := m.Called(resourceID, resourceType, context)
	return args.Error(0)
}

func (m *MockRecordRepository) GetAll() ([]repository.Record, error) {
	args := m.Called()
	return args.Get(0).([]repository.Record), args.Error(1)
}

func (m *MockRecordRepository) GetPaginated(continuationToken string, pageSize int) (*repository.PaginatedResult, error) {
	args := m.Called(continuationToken, pageSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult), args.Error(1)
}

// setupTestHandler creates a test handler with mock repository
func setupTestHandler() (*RecordHandler, *MockRecordRepository) {
	mockRepo := &MockRecordRepository{}
	handler := NewRecordHandler(mockRepo)
	return handler, mockRepo
}

// setupGinContext creates a test Gin context
func setupGinContext(method, url string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, url, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}

	c.Request = req
	return c, w
}

func TestNewRecordHandler(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	assert.NotNil(t, handler)
	assert.Equal(t, mockRepo, handler.repo)
}

func TestCreateRecord_Success(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	requestBody := CreateRecordRequest{
		ResourceID:   "user-123",
		ResourceType: "user",
		Context:      stringPtr(`{"action": "login"}`),
	}

	mockRepo.On("Insert", "user-123", "user", stringPtr(`{"action": "login"}`)).Return(nil)

	c, w := setupGinContext("POST", "/api/v1/records", requestBody)
	handler.CreateRecord(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Record created successfully", response["message"])
	assert.Equal(t, "user-123", response["resource_id"])
	assert.Equal(t, "user", response["resource_type"])

	mockRepo.AssertExpectations(t)
}

func TestCreateRecord_InvalidJSON(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	c, w := setupGinContext("POST", "/api/v1/records", nil)
	c.Request = httptest.NewRequest("POST", "/api/v1/records", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateRecord(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreateRecord_MissingRequiredFields(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	requestBody := CreateRecordRequest{
		ResourceID: "user-123",
		// Missing ResourceType
	}

	c, w := setupGinContext("POST", "/api/v1/records", requestBody)
	handler.CreateRecord(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreateRecord_RepositoryError(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	requestBody := CreateRecordRequest{
		ResourceID:   "user-123",
		ResourceType: "user",
	}

	mockRepo.On("Insert", "user-123", "user", (*string)(nil)).Return(errors.New("database error"))

	c, w := setupGinContext("POST", "/api/v1/records", requestBody)
	handler.CreateRecord(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Failed to create record", response["error"])

	mockRepo.AssertExpectations(t)
}

func TestGetRecords_Success(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	now := time.Now()
	mockRecords := []repository.Record{
		{
			ResourceID:   "user-123",
			ResourceType: "user",
			Context:      stringPtr(`{"action": "login"}`),
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ResourceID:   "doc-456",
			ResourceType: "document",
			Context:      nil,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	mockRepo.On("GetAll").Return(mockRecords, nil)

	c, w := setupGinContext("GET", "/api/v1/records", nil)
	handler.GetRecords(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "records")

	records := response["records"].([]any)
	assert.Len(t, records, 2)

	mockRepo.AssertExpectations(t)
}

func TestGetRecords_RepositoryError(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockRepo.On("GetAll").Return([]repository.Record{}, errors.New("database error"))

	c, w := setupGinContext("GET", "/api/v1/records", nil)
	handler.GetRecords(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Failed to retrieve records", response["error"])

	mockRepo.AssertExpectations(t)
}

func TestGetRecordsPaginated_Success(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	now := time.Now()
	mockRecords := []repository.Record{
		{
			ResourceID:   "user-123",
			ResourceType: "user",
			Context:      nil,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	token := "next-token"
	mockResult := &repository.PaginatedResult{
		Records:               mockRecords,
		NextContinuationToken: &token,
	}

	mockRepo.On("GetPaginated", "", 5).Return(mockResult, nil)

	c, w := setupGinContext("GET", "/api/v1/records/paginated", nil)
	handler.GetRecordsPaginated(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response repository.PaginatedResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Records, 1)
	assert.NotNil(t, response.NextContinuationToken)
	assert.Equal(t, "next-token", *response.NextContinuationToken)

	mockRepo.AssertExpectations(t)
}

func TestGetRecordsPaginated_WithCustomPageSize(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockResult := &repository.PaginatedResult{
		Records:               []repository.Record{},
		NextContinuationToken: nil,
	}

	mockRepo.On("GetPaginated", "", 10).Return(mockResult, nil)

	c, w := setupGinContext("GET", "/api/v1/records/paginated?page_size=10", nil)
	handler.GetRecordsPaginated(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetRecordsPaginated_WithContinuationToken(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	token := "test-token"
	mockResult := &repository.PaginatedResult{
		Records:               []repository.Record{},
		NextContinuationToken: nil,
	}

	mockRepo.On("GetPaginated", token, 5).Return(mockResult, nil)

	c, w := setupGinContext("GET", "/api/v1/records/paginated?continuation_token="+token, nil)
	handler.GetRecordsPaginated(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetRecordsPaginated_InvalidPageSize(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockResult := &repository.PaginatedResult{
		Records:               []repository.Record{},
		NextContinuationToken: nil,
	}

	// Should default to 5 when invalid page size is provided
	mockRepo.On("GetPaginated", "", 5).Return(mockResult, nil)

	c, w := setupGinContext("GET", "/api/v1/records/paginated?page_size=invalid", nil)
	handler.GetRecordsPaginated(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetRecordsPaginated_PageSizeLimit(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockResult := &repository.PaginatedResult{
		Records:               []repository.Record{},
		NextContinuationToken: nil,
	}

	// Should cap at 100 when page size exceeds limit
	mockRepo.On("GetPaginated", "", 100).Return(mockResult, nil)

	c, w := setupGinContext("GET", "/api/v1/records/paginated?page_size=150", nil)
	handler.GetRecordsPaginated(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetRecordsPaginated_RepositoryError(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockRepo.On("GetPaginated", "", 5).Return((*repository.PaginatedResult)(nil), errors.New("invalid token"))

	c, w := setupGinContext("GET", "/api/v1/records/paginated", nil)
	handler.GetRecordsPaginated(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "invalid token")

	mockRepo.AssertExpectations(t)
}

func TestCreateRecordFromQuery_Success(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockRepo.On("Insert", "user-123", "user", stringPtr("test-context")).Return(nil)

	c, w := setupGinContext("POST", "/api/v1/records/create?resource_id=user-123&resource_type=user&context=test-context", nil)
	handler.CreateRecordFromQuery(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Record created successfully", response["message"])
	assert.Equal(t, "user-123", response["resource_id"])
	assert.Equal(t, "user", response["resource_type"])

	mockRepo.AssertExpectations(t)
}

func TestCreateRecordFromQuery_WithoutContext(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockRepo.On("Insert", "doc-456", "document", (*string)(nil)).Return(nil)

	c, w := setupGinContext("POST", "/api/v1/records/create?resource_id=doc-456&resource_type=document", nil)
	handler.CreateRecordFromQuery(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreateRecordFromQuery_MissingResourceID(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	c, w := setupGinContext("POST", "/api/v1/records/create?resource_type=user", nil)
	handler.CreateRecordFromQuery(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "resource_id query parameter is required", response["error"])

	mockRepo.AssertExpectations(t)
}

func TestCreateRecordFromQuery_MissingResourceType(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	c, w := setupGinContext("POST", "/api/v1/records/create?resource_id=user-123", nil)
	handler.CreateRecordFromQuery(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "resource_type query parameter is required", response["error"])

	mockRepo.AssertExpectations(t)
}

func TestCreateRecordFromQuery_RepositoryError(t *testing.T) {
	handler, mockRepo := setupTestHandler()

	mockRepo.On("Insert", "user-123", "user", (*string)(nil)).Return(errors.New("database error"))

	c, w := setupGinContext("POST", "/api/v1/records/create?resource_id=user-123&resource_type=user", nil)
	handler.CreateRecordFromQuery(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Failed to create record", response["error"])

	mockRepo.AssertExpectations(t)
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}