# URL Shortener in Go

A high-performance URL shortener backend built with Go, featuring advanced caching, rate limiting, and analytics.

## Features

- **Caching URL Redirection** - Fast, cached URL lookups for optimal performance
- **Rate Limiting** - Sliding window technique with Valkey cache to prevent abuse
- **Prefork Load Balancing** - Efficient request distribution across multiple processes
- **Async Analytics Collection** - Non-blocking analytics tracking
- **Graceful Shutdown** - Clean, safe application shutdown
- **Comprehensive Makefile** - Easy-to-use build and deployment commands
- **Strict Request Validation** - Robust input validation for all endpoints

## Technology Stack

- **Go Fiber** - HTTP router framework
- **PostgreSQL** - Primary data store
- **Valkey** - In-memory cache
- **GNU Make** - Build automation
- **Sqlc** - Type-safe SQL query generation
- **Docker Compose** - Local development environment

## Getting Started

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 14+
- Valkey/Redis
- Docker & Docker Compose (optional)

### Local Development

```bash
# Install dependencies
make install

# Run the application
make run

# Run tests
make test

# Build the project
make build
```

For more commands, see the Makefile.
