package repository

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Record struct {
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type"`
	Context      *string   `json:"context,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PaginatedResult struct {
	Records           []Record `json:"records"`
	NextContinuationToken *string  `json:"next_continuation_token,omitempty"`
}

const DefaultPageSize = 5

type RecordRepository struct {
	db *sql.DB
}

// NewRecordRepository creates and returns a new RecordRepository instance.
// It takes a database connection and returns a repository for managing
// record operations including CRUD and pagination functionality.
func NewRecordRepository(db *sql.DB) *RecordRepository {
	return &RecordRepository{db: db}
}

// CreateTable creates the resource_context table if it doesn't already exist.
// The table includes resource_id (varchar), resource_type (varchar), context (longtext),
// created_at and updated_at (timestamp) columns with a composite primary key on
// (resource_type, resource_id). If the old table structure exists, it drops and recreates it.
func (r *RecordRepository) CreateTable() error {
	// Drop the old table if it exists to handle schema migration
	dropQuery := "DROP TABLE IF EXISTS resource_context"
	if _, err := r.db.Exec(dropQuery); err != nil {
		return err
	}

	// Create the new table with updated schema
	createQuery := `
	CREATE TABLE resource_context (
		resource_id varchar(128) not null,
		resource_type varchar(128) not null,
		context longtext default null,
		created_at timestamp not null,
		updated_at timestamp not null,
		PRIMARY KEY (resource_type, resource_id)
	)`

	_, err := r.db.Exec(createQuery)
	return err
}

// Insert adds a new record to the database with the specified fields.
// Both created_at and updated_at are set to the current time.
// Returns an error if the insertion fails or if a record with the same
// composite key (resource_type, resource_id) already exists.
func (r *RecordRepository) Insert(resourceID, resourceType string, context *string) error {
	now := time.Now()
	query := "INSERT INTO resource_context (resource_id, resource_type, context, created_at, updated_at) VALUES (?, ?, ?, ?, ?)"
	_, err := r.db.Exec(query, resourceID, resourceType, context, now, now)
	return err
}

// GetAll retrieves all records from the database ordered by created_at descending.
// This method returns all records without pagination and is useful for
// getting a complete dataset or when pagination is not needed.
func (r *RecordRepository) GetAll() ([]Record, error) {
	query := "SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context ORDER BY created_at DESC"
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		err := rows.Scan(&record.ResourceID, &record.ResourceType, &record.Context, &record.CreatedAt, &record.UpdatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

// encodeContinuationToken creates a base64-encoded token from the last record's data.
// The token contains the resource_type, resource_id, and timestamp (as Unix timestamp)
// separated by pipe characters. This token is used for cursor-based pagination to
// determine where the next page should start.
func (r *RecordRepository) encodeContinuationToken(lastResourceType, lastResourceID string, lastCreatedAt time.Time) string {
	tokenData := fmt.Sprintf("%s|%s|%d", lastResourceType, lastResourceID, lastCreatedAt.Unix())
	return base64.URLEncoding.EncodeToString([]byte(tokenData))
}

// decodeContinuationToken parses a base64-encoded continuation token back into
// resource_type, resource_id, and timestamp values. It validates the token format
// and returns an error if the token is malformed or cannot be decoded. This is used
// to determine the starting point for the next page of results.
func (r *RecordRepository) decodeContinuationToken(token string) (string, string, time.Time, error) {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("invalid continuation token: %v", err)
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 3 {
		return "", "", time.Time{}, fmt.Errorf("invalid continuation token format")
	}

	resourceType := parts[0]
	resourceID := parts[1]

	timestamp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("invalid timestamp in token: %v", err)
	}

	return resourceType, resourceID, time.Unix(timestamp, 0), nil
}

// GetPaginated retrieves records using cursor-based pagination with continuation tokens.
// If continuationToken is empty, it returns the first page. Otherwise, it returns
// records that come after the position indicated by the token. The method fetches
// one extra record to determine if there are more pages available. Results are
// ordered by created_at DESC, resource_type DESC, resource_id DESC for consistent pagination.
func (r *RecordRepository) GetPaginated(continuationToken string, pageSize int) (*PaginatedResult, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	var query string
	var args []any

	if continuationToken == "" {
		query = "SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context ORDER BY created_at DESC, resource_type DESC, resource_id DESC LIMIT ?"
		args = []any{pageSize + 1}
	} else {
		lastResourceType, lastResourceID, lastCreatedAt, err := r.decodeContinuationToken(continuationToken)
		if err != nil {
			return nil, err
		}

		query = `SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context
				 WHERE (created_at < ? OR (created_at = ? AND resource_type < ?) OR (created_at = ? AND resource_type = ? AND resource_id < ?))
				 ORDER BY created_at DESC, resource_type DESC, resource_id DESC LIMIT ?`
		args = []any{lastCreatedAt, lastCreatedAt, lastResourceType, lastCreatedAt, lastResourceType, lastResourceID, pageSize + 1}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		err := rows.Scan(&record.ResourceID, &record.ResourceType, &record.Context, &record.CreatedAt, &record.UpdatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	result := &PaginatedResult{
		Records: records,
	}

	if len(records) > pageSize {
		result.Records = records[:pageSize]
		lastRecord := records[pageSize-1]
		token := r.encodeContinuationToken(lastRecord.ResourceType, lastRecord.ResourceID, lastRecord.CreatedAt)
		result.NextContinuationToken = &token
	}

	return result, nil
}