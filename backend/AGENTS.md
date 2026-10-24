# AGENTS.md

## 1. Project Overview

Develop a news website with a public-facing interface and an administrative panel.

The application must follow Clean Architecture, SOLID principles, and idiomatic Go practices. Prioritize simplicity, maintainability, testability, security, and clear separation of responsibilities.

### Core features

- Public news feed with pagination.
- News article pages.
- Administrative authentication and authorization.
- CRUD operations for news articles.
- News content editing, translation, and text improvement using an AI API.
- Image upload, storage, and editing for news articles.
- Image storage using an S3-compatible cloud service.
- PostgreSQL for persistent data storage.
- Docker-based local development and deployment.
- Future support for Kubernetes without introducing unnecessary infrastructure complexity now.

### Technology stack

- Language: Go.
- HTTP framework: Gin.
- Database: PostgreSQL.
- Database access: use a repository abstraction.
- Object storage: S3-compatible storage.
- Containerization: Docker and Docker Compose.
- Dependency injection: Uber FX when it provides a clear benefit.
- Future orchestration: Kubernetes.

Do not introduce additional frameworks or infrastructure without a concrete requirement.

## 2. Architecture and SOLID

Use Clean Architecture with clearly separated responsibilities.

Recommended package structure:

```text
cmd/
  main.go

internal/
  models/
    news.go
    errors.go

  service/
    news_service.go
    ai_service.go
    image_service.go

  repository/
    news_repository.go
    postgres/
      news_repository.go

  transport/
    http/
      handler/
      middleware/
      dto/
      router.go

  config/
  errs/
    errs.go

pkg/
  database/
    postgres/
  storage/
  ai/

migrations/
Dockerfile
docker-compose.yml
.env.example
AGENTS.md
```

This structure is a starting point, not a requirement to create every directory immediately. Adapt it when the implementation demonstrates a genuine need.

Notes:

- There is no separate `tests/` directory: tests live next to the code they cover, in the same package (standard Go layout).
- Deployment assets live next to the backend sources or are defined later; do not create infrastructure directories in advance.

### Dependency rules

- Domain must not depend on Gin, PostgreSQL, S3 SDKs, or external AI providers.
- Services contain business logic and depend on interfaces rather than infrastructure implementations.
- Repositories implement persistence operations and must not contain business logic.
- HTTP handlers translate HTTP requests into service calls and service results into HTTP responses.
- External integrations belong in `pkg/` when they are intended to be reusable, or in an appropriate internal package when they are application-specific.
- Dependency direction must point inward toward domain and business logic.
- Define interfaces close to the code that consumes them.
- Use dependency injection to assemble implementations at application startup.

### SOLID requirements

- **S — Single Responsibility:** each component should have one clear responsibility.
- **O — Open/Closed:** extend behavior through appropriate abstractions rather than modifying unrelated code.
- **L — Liskov Substitution:** implementations must respect their interfaces' contracts.
- **I — Interface Segregation:** prefer small, focused interfaces over large general-purpose interfaces.
- **D — Dependency Inversion:** high-level business logic must not depend on concrete infrastructure.

Avoid unnecessary abstractions, excessive interfaces, and speculative generalization. Apply SOLID where it improves the design.

## 3. Go Coding Standards

Follow idiomatic Go and the official Go Code Review Comments and Effective Go guidance where applicable.

- Prefer simple, readable implementations.
- Use explicit error handling.
- Avoid unnecessary abstractions, global mutable state, and hidden side effects.
- Use `context.Context` for request-scoped cancellation and deadlines where appropriate.
- Pass `context.Context` as the first parameter of functions that require it.
- Do not store request contexts in long-lived structs.
- Avoid classic OOP patterns when ordinary Go composition and interfaces are sufficient.
- Use meaningful names and keep functions focused.
- Use constructor functions for initializing components.
- Keep exported APIs minimal.
- Run `gofmt` on all Go source files.
- Do not add dependencies without a clear reason.

### Function parameter formatting

If a function has more than four parameters, place each parameter on a separate line.

This rule applies to function declarations, method declarations, and function literals.

Correct:

```go
func (s *NewsService) Create(
    ctx context.Context,
    title string,
    content string,
    language string,
    imageURL string,
    authorID int64,
) error {
    return nil
}
```

Incorrect:

```go
func (s *NewsService) Create(ctx context.Context, title string, content string, language string, imageURL string, authorID int64) error {
    return nil
}
```

For four or fewer parameters, use standard Go formatting and prioritize readability.

Do not introduce parameter structs solely to avoid this formatting rule. Use a parameter struct when the values form a meaningful input object or when it improves the API.

### Constants

Use constants instead of magic numbers and repeated literal values when they represent meaningful configuration, limits, statuses, timeouts, or business rules.

Do not introduce constants for every incidental literal when doing so would reduce readability.

### Comments

Keep code comments short: a single line, at most 2–3 lines. Write all comments in English.

## 4. Function Context and Error Wrapping

Every function in the service and repository layers must declare a function-context constant at the beginning of its body.

Use the following naming convention:

```go
func (s *NewsService) Create(
    ctx context.Context,
    input CreateNewsInput,
) error {
    const fc = "newsService_Create"

    // Implementation.
    return nil
}
```

Repository example:

```go
func (r *NewsRepo) Create(
    ctx context.Context,
    news *models.News,
) error {
    const fc = "newsRepo_Create"

    // Implementation.
    return nil
}
```

Requirements:

- Use `serviceName_functionName` for service functions.
- Use `repoName_functionName` for repository functions.
- Keep the `fc` constant local to the function.
- Use the constant as the operation identifier for error wrapping.
- Preserve the original error using `%w` or the project's established wrapping helper.
- Do not log errors in repositories.
- Do not add function-context constants to HTTP handlers or unrelated packages unless explicitly required.

Example:

```go
if err := r.db.Create(news).Error; err != nil {
    return fmt.Errorf("%s: create news: %w", fc, err)
}
```

Use the project's established error-wrapping conventions consistently.

If a function returns multiple values, preserve the appropriate zero values when returning an error.

## 5. Error Handling

Maintain a centralized, explicit approach to internal and client-facing errors.

Use `internal/errs/errs.go` as the canonical location for application error definitions. If the project already has an established error package, extend it instead of creating a duplicate.

The error package should distinguish between:

- Validation errors.
- Not-found errors.
- Conflict errors.
- Unauthorized errors.
- Forbidden errors.
- Internal errors.
- External dependency failures.
- Context cancellation and deadline expiration where relevant.

Prefer sentinel errors or typed errors when callers need to classify failures.

Example:

```go
package errs

import "errors"

var (
    ErrNewsNotFound = errors.New("news not found")
    ErrInvalidInput = errors.New("invalid input")
    ErrConflict     = errors.New("resource conflict")
)
```

Rules:

- Do not expose raw database errors, SQL details, internal stack traces, credentials, or external provider responses to clients.
- Preserve error causes when wrapping errors.
- Use `errors.Is` and `errors.As` to classify errors.
- Translate application errors into appropriate HTTP status codes at the transport boundary.
- Return a generic internal error response for unexpected failures.
- Keep error classification separate from HTTP response formatting.
- Never convert every error into a generic internal error if it has a known, meaningful classification.
- Do not silently ignore errors.

The HTTP layer must return consistent JSON error responses.

```json
{
  "error": {
    "message": "News article not found"
  }
}
```

The error payload carries a client-facing message only; clients classify failures by the HTTP status code. Client-facing messages must not disclose internal implementation details.

The mapping from application errors to responses lives in one place — the `errResps` table in `internal/transport/http/handler/errors.go`. Each entry maps a sentinel error onto its HTTP status and message (`ErrResp`). Add new client-facing errors as a single table entry; do not branch on error types inside handlers.

## 6. Logging

Application logging must occur only in the service layer.

Repositories, HTTP handlers, and infrastructure adapters must not independently log application errors.

Rules:

- Use structured logging.
- Use the project's chosen logger consistently; prefer Go's `log/slog` unless an existing project convention dictates otherwise.
- Include the `fc` identifier when recording a failed operation.
- Wrap and propagate errors from repositories and external integrations to services.
- Services are responsible for logging failures at the appropriate level.
- Avoid logging the same error repeatedly at every layer.
- Do not log passwords, authentication tokens, API keys, full sensitive payloads, or other secrets.
- Include useful contextual identifiers when available, such as article ID or operation name.
- Keep logs actionable and avoid excessive noise.

Example:

```go
const fc = "newsService_Create"

if err := s.repo.Create(ctx, news); err != nil {
    wrappedErr := fmt.Errorf("%s: %w", fc, err)

    s.logger.ErrorContext(
        ctx,
        "failed to create news article",
        "fc", fc,
        "error", wrappedErr,
    )

    return wrappedErr
}
```

This is illustrative. Adapt the logger interface and error-returning conventions to the actual project.

If a failure is already logged and propagated by a lower-level service, do not log it again unless the current layer adds meaningful operational context.

## 7. Database and Repository Layer

PostgreSQL is the primary relational database.

Requirements:

- All database access must be encapsulated in the repository layer.
- Use migrations for schema changes.
- Never modify the production schema manually as a substitute for migrations.
- Use parameterized queries or safe query-builder/ORM APIs.
- Never concatenate untrusted input into SQL.
- Pass contexts to database operations.
- Use transactions for operations requiring atomicity.
- Handle unique constraints and foreign-key violations appropriately.
- Use database constraints to protect important invariants.
- Implement pagination for news listings.
- Avoid N+1 query patterns.
- Select only the data needed for an operation where practical.
- Do not return database-specific types to the domain or transport layers unless there is a justified reason.

Repositories must return domain-level results and errors, not HTTP responses.

Repository methods should expose operations needed by business logic rather than mirror every database operation mechanically.

## 8. News Management and CRUD

The administrative interface must support creating, reading, updating, and deleting news articles.

A news article may include:

- ID.
- Title.
- Slug.
- Summary or excerpt.
- Main content.
- Original language.
- Target language or translation metadata, where applicable.
- Publication status.
- Cover image reference.
- Author or editor metadata, where required.
- Creation and update timestamps.
- Publication timestamp, when published.

Treat this as an initial model, not a mandatory final schema. Define fields based on actual product requirements.

Requirements:

- Validate all incoming data.
- Generate or validate unique slugs.
- Prevent unauthorized access to administrative operations.
- Separate draft management from public publication.
- Do not expose drafts through public endpoints.
- Use pagination and deterministic ordering.
- Define deletion behavior explicitly; use soft deletion only if product requirements justify it.
- Keep public DTOs separate from administrative DTOs when their data or permissions differ.

Business rules belong in the service layer.

## 9. AI Integration

The application will integrate with an external AI API to help editors translate, improve, and edit news content.

AI capabilities may include:

- Translation between supported languages.
- Grammar and spelling correction.
- Style and readability improvement.
- Headline and summary suggestions.
- Optional image-editing or image-generation workflows when supported by the selected provider.

Requirements:

- Encapsulate the AI provider behind an application-facing interface.
- Keep provider-specific SDKs and HTTP clients outside the domain layer.
- Configure API keys through environment variables or a secrets manager.
- Apply timeouts and context cancellation.
- Handle rate limits, provider errors, malformed responses, and unavailable services.
- Validate AI output before returning it to the editor.
- Do not automatically publish AI-generated content.
- Preserve the original article unless the editor explicitly saves the suggested changes.
- Allow the editor to review and accept or reject generated content.
- Avoid sending unnecessary sensitive data to external providers.
- Do not hardcode prompts, model identifiers, or provider-specific assumptions throughout business logic.

AI-generated content is a suggestion, not an authoritative source. Preserve factual meaning and flag uncertain changes rather than silently inventing facts.

For image editing, define the provider capabilities and supported operations before implementing provider-specific functionality. Keep original images unless the user explicitly replaces them.

## 10. Image Storage

Store image files in an S3-compatible object storage service. Store image metadata and object references in PostgreSQL, not the binary contents themselves.

Requirements:

- Encapsulate storage operations behind an interface.
- Keep the implementation independent of a specific S3-compatible vendor.
- Configure bucket names, endpoints, regions, and credentials externally.
- Validate file types and sizes.
- Do not trust client-provided MIME types alone.
- Generate safe object keys rather than using arbitrary user-provided filenames.
- Prevent unauthorized uploads and modifications.
- Do not expose secret storage credentials to the frontend.
- Use private storage with controlled access or explicitly configured public delivery.
- Handle failed uploads and database writes without leaving unmanaged objects where practical.
- Define cleanup behavior for replaced and deleted images.
- Support appropriate image transformations or thumbnails when required.

The public API should expose an image URL or a controlled image-delivery endpoint, not internal storage credentials.

## 11. HTTP API and Security

Use Gin exclusively in the HTTP transport layer.

Requirements:

- Keep handlers thin.
- Validate request bodies, path parameters, query parameters, and pagination values.
- Use explicit request and response DTOs.
- Return consistent JSON responses.
- Apply centralized error-to-HTTP mapping.
- Add request size limits.
- Configure CORS explicitly for the intended frontend origins.
- Use appropriate security headers.
- Protect administrative routes with authentication and authorization middleware.
- Store passwords using a suitable password-hashing algorithm if password-based authentication is implemented.
- Use secure session cookies or properly validated tokens according to the chosen authentication design.
- Rate-limit sensitive endpoints where appropriate.
- Never commit secrets or real credentials.
- Use environment variables for deployment-specific configuration.
- Avoid exposing internal errors or stack traces in HTTP responses.

Do not implement authentication by trusting a user ID or role supplied directly by the client.

## 12. Dependency Injection and Application Startup

Uber FX may be used for dependency injection and lifecycle management.

Use it when it simplifies dependency assembly, startup, shutdown, and testing.

Requirements:

- Keep constructors explicit.
- Register providers and invoke functions in a clear, predictable manner.
- Group dependencies into meaningful modules when the application grows.
- Use FX lifecycle hooks for resources that require startup or shutdown management.
- Close database connections and other resources gracefully.
- Avoid circular dependencies.
- Do not use FX merely to avoid passing dependencies explicitly.
- Keep business logic independent of FX.

If the initial application remains small, explicit constructors are acceptable. Introduce FX when the complexity justifies it without changing domain logic.

## 13. Docker and Deployment

Provide a reproducible local development environment using Docker and Docker Compose.

Requirements:

- Use a multi-stage Docker build for the Go application where appropriate.
- Keep images minimal and avoid unnecessary build tools in the runtime image.
- Run as a non-root user where practical.
- Use environment variables for configuration.
- Add health checks where useful.
- Persist PostgreSQL data using a named volume in local development.
- Do not store production secrets in Docker images or committed Compose files.
- Support graceful application shutdown.
- Keep application configuration independent of a specific deployment environment.

Kubernetes is a future deployment target.

Do not add Kubernetes manifests, Helm charts, operators, service meshes, or other orchestration components until there is a concrete deployment requirement.

## 14. Testing

Write tests for business logic and critical application behavior.

Requirements:

- Unit-test services independently of PostgreSQL, Gin, S3, and AI providers.
- Use mocks or small test doubles for external dependencies when appropriate.
- Test error paths as well as successful execution.
- Add repository integration tests against PostgreSQL for important queries and transaction behavior.
- Test HTTP status codes, response formats, validation, and authorization.
- Test error mapping for known and unexpected failures.
- Test AI integrations using deterministic fixtures or mocked provider responses.
- Test image upload and cleanup behavior.
- Avoid brittle tests that depend on external services or nondeterministic AI output.
- Run `go test ./...` before considering a task complete.

Do not mock every dependency automatically. Use real implementations in integration tests when doing so improves confidence.

## 15. Code Quality and Development Workflow

Before implementing a feature:

1. Inspect the existing project structure and conventions.
2. Identify the relevant domain rules and interfaces.
3. Define or update the required models and DTOs.
4. Implement business logic in the service layer.
5. Implement persistence or external integrations behind interfaces.
6. Connect the implementation through the transport layer.
7. Add tests for expected behavior and failure cases.
8. Run formatting, tests, and relevant static analysis.

Recommended checks:

```bash
gofmt -w .
go test ./...
go vet ./...
```

Use a project-appropriate linter if configured. Do not run formatting or automated fixes in ways that unintentionally rewrite unrelated files.

Additional rules:

- Work according to the superpowers methodology: for every task, follow its skills — `brainstorming` before designing features, `writing-plans` for non-trivial work, `executing-plans` with `test-driven-development` during implementation, `verification-before-completion` before declaring done, and `requesting-code-review` for the final review. Do not skip stages or move to the next one before the current is complete.
- Use the project's skills (golang-* from `.opencode/skills`) to improve code quality during development: style, error handling, testing, security, performance. Rely on them first, before general knowledge.
- Do not rewrite unrelated code.
- Do not silently change existing API contracts.
- Do not introduce new dependencies without explaining their purpose.
- Do not claim tests passed unless they were actually executed.
- Update documentation and `.env.example` when configuration changes.
- Keep commits and changes focused on the requested task.
- Ask for clarification when a requirement materially affects architecture, security, or data integrity and cannot reasonably be inferred.

### Git rules

- Commit only when explicitly commanded. Never commit on your own initiative.
- Push only when explicitly commanded. Never push on your own initiative.
- Start every commit message with a type prefix:
  - `[+]` — adding new functionality or files.
  - `[*]` — refactoring or changes to existing code without changing behavior.
  - `[-]` — deleting code or files.

## 16. Constants and Configuration

Use named constants for meaningful fixed values.

Examples include:

- Default pagination limits.
- Maximum upload sizes.
- Supported statuses.
- Application-specific error codes.
- Retry limits.
- Operation timeouts when they are fixed by application policy.

Use configuration for values that vary by environment or deployment.

Do not hardcode:

- Database credentials.
- AI API keys.
- S3 credentials.
- Deployment-specific endpoints.
- Production bucket names.
- Environment-specific URLs.

Avoid scattering duplicate values throughout the codebase.

## 17. Future Extensibility

Keep the design ready for future enhancements without implementing them prematurely.

Possible future capabilities:

- Kubernetes deployment.
- Background processing for AI operations.
- Scheduled article publication.
- Multiple AI providers.
- Multiple image storage providers.
- Caching and CDN integration.
- Editorial roles and permissions.
- Audit history for article edits.
- Additional languages.

Introduce queues, background workers, caching, or additional services only when the requirements justify them.

Prefer a well-structured modular monolith over microservices at the initial stage.

## 18. Final Principles

When making implementation decisions, prioritize these goals in order:

1. Correctness and data integrity.
2. Security and privacy.
3. Simplicity and readability.
4. Testability and maintainability.
5. Clear architectural boundaries.
6. Performance where justified by evidence.
7. Extensibility for demonstrated requirements.

The goal is not to maximize abstractions or infrastructure. The goal is to build a reliable news platform whose code remains easy to understand, test, and extend.
