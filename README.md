# URL Shortener

URL Shortener Is A Link Management And Click Analytics Backend API.

## Run the Application

```sh
docker compose up
make goose-up-all
make run
```

The API listens on `:3000`. Local development defaults live in the committed `.env`.

## API Documentation

- Swagger UI: <http://localhost:3000/swagger>
- OpenAPI document: <http://localhost:3000/swagger/openapi.json>

## Functional Features

### Authentication & Accounts
- **Registration:** Name, username, and password with strict validation (username charset `[A-Za-z0-9_]`, length 1–32; password length 8–50; matching password confirmation).
- **Sessions:** Login/logout using HTTP-only `session_id` and `session_token` cookies with a separate `csrf_token` cookie and a 120-day expiry.
- **CSRF Protection:** Mutating endpoints require the `csrf_token` cookie to match the token stored with the session.
- **Username Changes:** Change your username, with uniqueness serialized through a Redis lock.
- **Password Changes:** Change your password after verifying the current one; all other sessions are signed out.
- **Account Management:** View the current account, or delete it (soft delete plus session removal).
- **Session Cleanup:** A weekly background job removes expired and soft-deleted sessions.

### Link Management
- **Short Links:** Create links with a destination URL, title, and optional expiry.
- **Alias Generation:** Random aliases start at 5 letters and grow automatically when the current length is exhausted.
- **Listing:** Paginated listing of your links with sorting by `title`, `url`, `alias`, or `created_at` (newest first by default), plus title search.
- **Editing:** Update the title, expiry, or active status. Destination, alias, and owner are immutable.
- **Soft Deletion:** Deleted links keep their alias reserved and are excluded from management endpoints.

### Public Redirects
- **Resolution:** `GET /r/:alias` redirects to the destination.
- **Status Handling:** Unknown aliases return `404 Not Found`; expired or inactive links return `410 Gone`.
- **Click Recording:** Each click records the client IP, browser, and operating system extracted from the `User-Agent`, buffered for asynchronous persistence.

### Analytics
- **Redirection Listings:** Paginated redirection records for the current account or for one of its links, plus single-record lookup.
- **Sorting:** Sort by `created_at` (newest first by default), `os`, or `platform`, in either direction, with a stable tiebreaker.
- **Click Counts:** Link click counters are updated asynchronously through a Redis-backed event queue.

### Misc
- **Health Checks:** Liveness and readiness endpoints; readiness verifies all services.

## Non-Functional Features

### Performance & Scalability
- **Cursor Pagination:** Keyset-based pagination with opaque cursors, bounded limits (1–100, default 10), and strict cursor validation.
- **Batched Click Writes:** Redirections are buffered and written in batches (up to 500 records) or on a spaced timer, reducing database round trips.
- **Concurrent Event Consumers:** Four workers consume URL-click events in parallel.
- **Connection Pooling:** Tuned PostgreSQL pool (max open/idle connections, lifetimes).
- **Indexed Lookups:** Indexes on link ownership, aliases, redirection ownership, and link redirections.

### Reliability
- **Background Workers:** Scheduled jobs with retries, decorrelated-jitter backoff, timeouts, and single-instance execution through Redis distributed locks.
- **Graceful Shutdown:** Signal handling stops workers and drains buffered redirections before exiting.
- **Distributed Locking:** Redis read/write locks serialize account deletion against in-flight authenticated requests and protect username uniqueness.
- **Soft Deletion:** Accounts and links are retained; aliases are never reused, and redirections outlive the links they reference.
- **Consistent Errors:** Stable error kinds and JSON error payloads across endpoints.

### Security
- **Password Security:** bcrypt hashing with password length rules.
- **Session Cookies:** HTTP-only session cookies plus a separate CSRF token cookie.
- **Owner-Scoped Access:** Every link and analytics read is scoped to the authenticated account.
- **Input Validation:** Request payloads, path parameters, and query parameters are validated on every endpoint.
- **Request Isolation:** Authenticated requests hold a shared per-user lock so account deletion cannot race them.

### Observability
- **Request Logging:** Structured logs with method, path, status, duration, and client IP.
- **Service-Scoped Logging:** Grouped structured logging (slog) per service and worker.

### Architecture & Tooling
- **Modular Services:** Isolated domains (auth, shortening, redirection, postgres, redis), each with its own schema, goose migrations, and sqlc-generated type-safe queries.
- **Lifecycle Management:** Central service manager for coordinated start/stop and dependency injection.
- **Declarative API Specs:** OpenAPI documentation generated from route specifications and served through Swagger UI.
- **Docker Compose Dev Stack:** PostgreSQL and Redis with a Makefile-driven workflow (`make build`, `make run`, goose targets, CLI helpers).
- **Testability:** Parallel integration tests with testcontainers (PostgreSQL, Redis), a migrated template database, per-test databases and Redis databases, plus unit tests for utilities and buffering.

## Tech Stack

- **Backend:** Go 1.27 ([moon](https://github.com/assaidy/moon))
- **Database:** PostgreSQL 18 (with sqlc for type-safe queries and goose for migrations)
- **Cache, Queue & Locks:** Redis 8 (go-redis) with distributed locking
- **Authentication:** bcrypt, session cookies, and CSRF tokens
- **Validation:** ozzo-validation
- **Background Jobs:** <github.com/assaidy/workers>
- **API Documentation:** OpenAPI / Swagger UI
- **Testing:** testify and testcontainers
- **Tooling:** Make, Docker Compose, sqlc, and goose
