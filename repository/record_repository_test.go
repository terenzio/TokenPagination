package repository

import (
	"database/sql"
	"encoding/base64"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a mock database connection for testing
func setupTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *RecordRepository) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := NewRecordRepository(db)
	return db, mock, repo
}

func TestNewRecordRepository(t *testing.T) {
	db, _, repo := setupTestDB(t)
	defer db.Close()

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestCreateTable(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	// Expect DROP TABLE query first
	mock.ExpectExec("DROP TABLE IF EXISTS resource_context").WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect CREATE TABLE query
	mock.ExpectExec(`CREATE TABLE resource_context \(
		resource_id varchar\(128\) not null,
		resource_type varchar\(128\) not null,
		context longtext default null,
		created_at timestamp not null,
		updated_at timestamp not null,
		PRIMARY KEY \(resource_type, resource_id\)
	\)`).WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.CreateTable()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateTable_Error(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	// Expect DROP TABLE to succeed
	mock.ExpectExec("DROP TABLE IF EXISTS resource_context").WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect CREATE TABLE to fail
	mock.ExpectExec(`CREATE TABLE resource_context`).WillReturnError(assert.AnError)

	err := repo.CreateTable()
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	resourceID := "user-123"
	resourceType := "user"
	context := `{"action": "login"}`

	mock.ExpectExec(`INSERT INTO resource_context \(resource_id, resource_type, context, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs(resourceID, resourceType, &context, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Insert(resourceID, resourceType, &context)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_WithNilContext(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	resourceID := "doc-456"
	resourceType := "document"

	mock.ExpectExec(`INSERT INTO resource_context \(resource_id, resource_type, context, created_at, updated_at\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs(resourceID, resourceType, nil, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Insert(resourceID, resourceType, nil)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_Error(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	resourceID := "user-123"
	resourceType := "user"

	mock.ExpectExec(`INSERT INTO resource_context`).
		WillReturnError(assert.AnError)

	err := repo.Insert(resourceID, resourceType, nil)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAll(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	context1 := `{"action": "login"}`

	rows := sqlmock.NewRows([]string{"resource_id", "resource_type", "context", "created_at", "updated_at"}).
		AddRow("user-123", "user", &context1, now, now).
		AddRow("doc-456", "document", nil, now, now)

	mock.ExpectQuery(`SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context ORDER BY created_at DESC`).
		WillReturnRows(rows)

	records, err := repo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, records, 2)

	assert.Equal(t, "user-123", records[0].ResourceID)
	assert.Equal(t, "user", records[0].ResourceType)
	assert.Equal(t, &context1, records[0].Context)

	assert.Equal(t, "doc-456", records[1].ResourceID)
	assert.Equal(t, "document", records[1].ResourceType)
	assert.Nil(t, records[1].Context)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAll_Error(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context`).
		WillReturnError(assert.AnError)

	records, err := repo.GetAll()
	assert.Error(t, err)
	assert.Nil(t, records)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEncodeContinuationToken(t *testing.T) {
	db, _, repo := setupTestDB(t)
	defer db.Close()

	resourceType := "user"
	resourceID := "user-123"
	createdAt := time.Unix(1234567890, 0)

	token := repo.encodeContinuationToken(resourceType, resourceID, createdAt)
	assert.NotEmpty(t, token)

	// Verify we can decode it back
	decodedType, decodedID, decodedTime, err := repo.decodeContinuationToken(token)
	assert.NoError(t, err)
	assert.Equal(t, resourceType, decodedType)
	assert.Equal(t, resourceID, decodedID)
	assert.Equal(t, createdAt.Unix(), decodedTime.Unix())
}

func TestDecodeContinuationToken_InvalidBase64(t *testing.T) {
	db, _, repo := setupTestDB(t)
	defer db.Close()

	_, _, _, err := repo.decodeContinuationToken("invalid-base64!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid continuation token")
}

func TestDecodeContinuationToken_InvalidFormat(t *testing.T) {
	db, _, repo := setupTestDB(t)
	defer db.Close()

	// Manually create invalid format (only 2 parts instead of 3)
	invalidData := base64.URLEncoding.EncodeToString([]byte("user|only-two-parts"))

	_, _, _, err := repo.decodeContinuationToken(invalidData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid continuation token format")
}

func TestGetPaginated_FirstPage(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	context1 := `{"action": "login"}`

	// Mock returns 6 rows (pageSize + 1) to test pagination
	rows := sqlmock.NewRows([]string{"resource_id", "resource_type", "context", "created_at", "updated_at"}).
		AddRow("user-1", "user", &context1, now, now).
		AddRow("user-2", "user", nil, now, now).
		AddRow("user-3", "user", nil, now, now).
		AddRow("user-4", "user", nil, now, now).
		AddRow("user-5", "user", nil, now, now).
		AddRow("user-6", "user", nil, now, now)

	mock.ExpectQuery(`SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context ORDER BY created_at DESC, resource_type DESC, resource_id DESC LIMIT \?`).
		WithArgs(6). // pageSize + 1
		WillReturnRows(rows)

	result, err := repo.GetPaginated("", 5)
	assert.NoError(t, err)
	assert.Len(t, result.Records, 5) // Should return only pageSize records
	assert.NotNil(t, result.NextContinuationToken) // Should have next token
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPaginated_WithToken(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	// Use a fixed time to avoid precision issues
	now := time.Unix(1234567890, 0)
	token := repo.encodeContinuationToken("user", "user-5", now)

	rows := sqlmock.NewRows([]string{"resource_id", "resource_type", "context", "created_at", "updated_at"}).
		AddRow("user-6", "user", nil, now, now)

	mock.ExpectQuery(`SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context WHERE \(created_at < \? OR \(created_at = \? AND resource_type < \?\) OR \(created_at = \? AND resource_type = \? AND resource_id < \?\)\) ORDER BY created_at DESC, resource_type DESC, resource_id DESC LIMIT \?`).
		WithArgs(now, now, "user", now, "user", "user-5", 6).
		WillReturnRows(rows)

	result, err := repo.GetPaginated(token, 5)
	assert.NoError(t, err)
	assert.Len(t, result.Records, 1)
	assert.Nil(t, result.NextContinuationToken) // No more pages
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPaginated_InvalidToken(t *testing.T) {
	db, _, repo := setupTestDB(t)
	defer db.Close()

	result, err := repo.GetPaginated("invalid-token", 5)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetPaginated_DefaultPageSize(t *testing.T) {
	db, mock, repo := setupTestDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"resource_id", "resource_type", "context", "created_at", "updated_at"})

	mock.ExpectQuery(`SELECT resource_id, resource_type, context, created_at, updated_at FROM resource_context ORDER BY created_at DESC, resource_type DESC, resource_id DESC LIMIT \?`).
		WithArgs(DefaultPageSize + 1).
		WillReturnRows(rows)

	result, err := repo.GetPaginated("", 0) // Invalid page size should use default
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}