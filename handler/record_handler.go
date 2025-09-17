package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"tokenpagination/repository"
)

// RecordRepositoryInterface defines the interface for record repository operations
type RecordRepositoryInterface interface {
	CreateTable() error
	Insert(resourceID, resourceType string, context *string) error
	GetAll() ([]repository.Record, error)
	GetPaginated(continuationToken string, pageSize int) (*repository.PaginatedResult, error)
}

type RecordHandler struct {
	repo RecordRepositoryInterface
}

// NewRecordHandler creates and returns a new RecordHandler instance.
// It takes a RecordRepositoryInterface and returns a handler for managing HTTP
// requests related to record operations including creation and retrieval.
func NewRecordHandler(repo RecordRepositoryInterface) *RecordHandler {
	return &RecordHandler{repo: repo}
}

type CreateRecordRequest struct {
	ResourceID   string  `json:"resource_id" binding:"required"`
	ResourceType string  `json:"resource_type" binding:"required"`
	Context      *string `json:"context,omitempty"`
}

// CreateRecord handles POST requests to create a new record from JSON payload.
// It expects a JSON body with resource_id, resource_type, and optional context fields
// and validates the input before inserting the record into the database. Returns 201
// on success or appropriate error status codes for validation or database failures.
func (h *RecordHandler) CreateRecord(c *gin.Context) {
	var req CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Insert(req.ResourceID, req.ResourceType, req.Context); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create record"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Record created successfully", "resource_id": req.ResourceID, "resource_type": req.ResourceType})
}

// GetRecords handles GET requests to retrieve all records from the database.
// This endpoint returns all records without pagination and is useful for
// getting the complete dataset. Results are ordered by created_at descending.
func (h *RecordHandler) GetRecords(c *gin.Context) {
	records, err := h.repo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve records"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"records": records})
}

// GetRecordsPaginated handles GET requests for paginated record retrieval.
// It supports continuation_token and page_size query parameters for cursor-based
// pagination. Page size is limited to 1-100 records with a default of 5.
// Returns records with an optional next_continuation_token for subsequent pages.
func (h *RecordHandler) GetRecordsPaginated(c *gin.Context) {
	continuationToken := c.Query("continuation_token")
	pageSize := 5

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			if ps > 100 {
				pageSize = 100 // Cap at 100
			} else {
				pageSize = ps
			}
		}
	}

	result, err := h.repo.GetPaginated(continuationToken, pageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CreateRecordFromQuery handles POST requests to create a record using query parameters.
// It expects resource_id and resource_type query parameters, with an optional context
// parameter. This provides an alternative to JSON-based record creation for simpler
// integrations or testing purposes.
func (h *RecordHandler) CreateRecordFromQuery(c *gin.Context) {
	resourceID := c.Query("resource_id")
	resourceType := c.Query("resource_type")
	contextStr := c.Query("context")

	if resourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource_id query parameter is required"})
		return
	}

	if resourceType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource_type query parameter is required"})
		return
	}

	var context *string
	if contextStr != "" {
		context = &contextStr
	}

	if err := h.repo.Insert(resourceID, resourceType, context); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create record"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Record created successfully", "resource_id": resourceID, "resource_type": resourceType})
}