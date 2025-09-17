# Token Pagination

A Go REST API application with MariaDB integration using Gin web framework. Features a layered architecture with repository and handler patterns.

## Running with Docker Compose

To build and run the application with MariaDB using Docker Compose:

```bash
# Build and start both the application and MariaDB
docker-compose up --build

# Run in detached mode
docker-compose up --build -d

# Stop the services
docker-compose down

# Clean up volumes (removes database data)
docker-compose down -v
```

The API will be available at `http://localhost:8080`

## Architecture

- **Repository Layer**: Handles database operations (`repository/record_repository.go`)
- **Handler Layer**: Manages HTTP requests and responses (`handler/record_handler.go`)
- **Main Application**: Sets up routes and starts the Gin server (`main.go`)

## API Endpoints

### Health Check
- `GET /health` - Check if the API is running

### Records Management
- `POST /api/v1/records` - Create a new record (JSON body)
- `GET /api/v1/records` - Retrieve all records
- `GET /api/v1/records/paginated` - Retrieve paginated records with continuation tokens
- `POST /api/v1/records/create` - Create a record using query parameters

### API Examples

#### Create Record (JSON)
```bash
curl -X POST http://localhost:8080/api/v1/records \
  -H "Content-Type: application/json" \
  -d '{
    "resource_id": "user-123",
    "resource_type": "user",
    "context": "{\"action\": \"login\", \"ip\": \"192.168.1.1\"}"
  }'
```

#### Create Record (Query Parameters)
```bash
curl -X POST "http://localhost:8080/api/v1/records/create?resource_id=doc-456&resource_type=document&context={\"title\": \"Project Plan\"}"
```

#### Get All Records
```bash
curl http://localhost:8080/api/v1/records
```

#### Get Paginated Records
```bash
# Get first page (5 records by default)
curl http://localhost:8080/api/v1/records/paginated

# Get first page with custom page size
curl "http://localhost:8080/api/v1/records/paginated?page_size=3"

# Get next page using continuation token from previous response
curl "http://localhost:8080/api/v1/records/paginated?continuation_token=MTIzNHwxNzM0NTY3ODkw"

# Custom page size with continuation token
curl "http://localhost:8080/api/v1/records/paginated?continuation_token=MTIzNHwxNzM0NTY3ODkw&page_size=10"
```

#### Health Check
```bash
curl http://localhost:8080/health
```

## Pagination with Continuation Tokens

This API implements **continuation token-based pagination** for efficient data retrieval. Unlike traditional offset-based pagination, continuation tokens provide several advantages:

### What is a Continuation Token?

A continuation token is a **base64-encoded string** that contains the position information needed to fetch the next page of results. In this implementation, the token encodes:
- `resource_type` of the last record in the current page
- `resource_id` of the last record in the current page
- `created_at` timestamp of the last record in the current page

### How Continuation Tokens Work

1. **First Request**: Call `/api/v1/records/paginated` without any token
2. **Response**: Returns records + `next_continuation_token` (if more pages exist)
3. **Next Request**: Use the `next_continuation_token` from the previous response
4. **End of Data**: When `next_continuation_token` is `null`, no more pages exist

### Example Pagination Flow

```json
# First request
GET /api/v1/records/paginated?page_size=3

{
  "records": [
    {
      "resource_id": "file-5123",
      "resource_type": "file",
      "context": "{\"name\": \"presentation.pptx\", \"slides\": 20}",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    },
    {
      "resource_id": "user-4890",
      "resource_type": "user",
      "context": "{\"action\": \"login\", \"device\": \"mobile\"}",
      "created_at": "2024-01-15T10:25:00Z",
      "updated_at": "2024-01-15T10:25:00Z"
    },
    {
      "resource_id": "task-4567",
      "resource_type": "task",
      "context": "{\"priority\": \"medium\", \"deadline\": \"2024-02-15\"}",
      "created_at": "2024-01-15T10:20:00Z",
      "updated_at": "2024-01-15T10:20:00Z"
    }
  ],
  "next_continuation_token": "dGFza3x0YXNrLTQ1Njd8MTcwNTM5ODQwMA=="
}

# Second request using the token
GET /api/v1/records/paginated?continuation_token=dGFza3x0YXNrLTQ1Njd8MTcwNTM5ODQwMA==&page_size=3

{
  "records": [
    {
      "resource_id": "doc-4234",
      "resource_type": "document",
      "context": "{\"title\": \"Technical Spec\", \"authors\": [\"alice\", \"bob\"]}",
      "created_at": "2024-01-15T10:15:00Z",
      "updated_at": "2024-01-15T10:15:00Z"
    }
  ]
  // No next_continuation_token = end of data
}
```

### Query Parameters

- `continuation_token` (optional): Token from previous response to get next page
- `page_size` (optional): Number of records per page (1-100, default: 5)

### Benefits of Continuation Tokens

- **Consistent Results**: No duplicate or missing records during pagination
- **Performance**: Efficient database queries using indexed columns
- **Real-time Safe**: Works correctly even when new records are added
- **Stateless**: No server-side pagination state to maintain

## Features

- REST API with Gin web framework
- MariaDB database integration with health checks
- Layered architecture (Repository + Handler pattern)
- **Continuation token-based pagination** for efficient data retrieval
- JSON and query parameter support for record creation
- Automatic table creation with proper schema
- Docker containerization with proper networking

## Database Schema

The application creates a `records` table with the following structure:
- `resource_id`: varchar(128) NOT NULL - stores the resource identifier (e.g., "user-123", "doc-456")
- `resource_type`: varchar(128) NOT NULL - stores the type of resource (e.g., "user", "document", "task", "file")
- `context`: longtext DEFAULT NULL - stores optional JSON context data with additional metadata
- `created_at`: timestamp NOT NULL - timestamp when the record was created
- `updated_at`: timestamp NOT NULL - timestamp when the record was last updated
- **Primary Key**: Composite key on (resource_type, resource_id)

The composite primary key ensures uniqueness across the combination of resource type and ID, allowing the same resource_id to exist for different resource types.