package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"tokenpagination/handler"
	"tokenpagination/repository"
)

// connectDB establishes a connection to the MariaDB database using environment variables.
// It reads database configuration from DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, and DB_NAME
// environment variables and returns a database connection with parseTime enabled for
// proper time handling.
func connectDB() (*sql.DB, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// setupRoutes configures and returns a Gin router with all API endpoints.
// It sets up the API routes for record management with the new schema,
// health checks, and enables release mode for production. The router includes
// both paginated and non-paginated endpoints for backward compatibility.
func setupRoutes(recordHandler *handler.RecordHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	api := r.Group("/api/v1")
	{
		api.POST("/records", recordHandler.CreateRecord)
		api.GET("/records", recordHandler.GetRecords)
		api.GET("/records/paginated", recordHandler.GetRecordsPaginated)
		api.POST("/records/create", recordHandler.CreateRecordFromQuery)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	return r
}

// SampleRecord represents a sample record to be loaded from the data file.
type SampleRecord struct {
	ResourceID   string
	ResourceType string
	Context      *string
}

// loadSampleData reads sample records from a text file and returns them as a slice.
// Each line in the file should contain resource_id|resource_type|context format.
// Empty lines are skipped, and parsing errors for individual lines are logged
// but don't stop the process.
func loadSampleData(filename string) ([]SampleRecord, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []SampleRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			log.Printf("Warning: Invalid format '%s': expected resource_id|resource_type|context", line)
			continue
		}

		record := SampleRecord{
			ResourceID:   parts[0],
			ResourceType: parts[1],
		}

		if len(parts) >= 3 && parts[2] != "" {
			record.Context = &parts[2]
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// populateSampleData inserts sample records into the database if it's empty.
// This function checks if any records already exist, and if not, loads sample
// data from 'sample_data.txt' and inserts each record with all required fields.
// This ensures the database has test data available immediately after startup.
func populateSampleData(repo *repository.RecordRepository) error {
	existingRecords, err := repo.GetAll()
	if err != nil {
		return err
	}

	if len(existingRecords) > 0 {
		fmt.Printf("Database already contains %d records, skipping sample data insertion\n", len(existingRecords))
		return nil
	}

	records, err := loadSampleData("sample_data.txt")
	if err != nil {
		return fmt.Errorf("failed to load sample data: %v", err)
	}

	fmt.Printf("Inserting %d sample records...\n", len(records))
	for _, record := range records {
		if err := repo.Insert(record.ResourceID, record.ResourceType, record.Context); err != nil {
			log.Printf("Warning: Failed to insert record %s/%s: %v", record.ResourceType, record.ResourceID, err)
		}
	}

	fmt.Println("Sample data insertion completed")
	return nil
}

// main is the entry point of the application.
// It establishes database connection, creates tables with the new schema,
// populates sample data, sets up HTTP routes, and starts the Gin web server
// on port 8080. The server provides REST API endpoints for record management
// with support for resource_id, resource_type, context fields and both
// traditional and paginated data retrieval.
func main() {
	fmt.Println("Starting application...")

	db, err := connectDB()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	recordRepo := repository.NewRecordRepository(db)
	if err := recordRepo.CreateTable(); err != nil {
		log.Fatal("Failed to create table:", err)
	}

	if err := populateSampleData(recordRepo); err != nil {
		log.Fatal("Failed to populate sample data:", err)
	}

	recordHandler := handler.NewRecordHandler(recordRepo)
	router := setupRoutes(recordHandler)

	fmt.Println("Server starting on port 8080...")
	fmt.Println("API endpoints:")
	fmt.Println("  POST /api/v1/records - Create record (JSON body)")
	fmt.Println("  GET  /api/v1/records - Get all records")
	fmt.Println("  GET  /api/v1/records/paginated - Get paginated records")
	fmt.Println("  POST /api/v1/records/create?resource_id=123&resource_type=user - Create record (query param)")
	fmt.Println("  GET  /health - Health check")

	if err := router.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}