# Project Guidelines

## Overview

Strautomagically is a Strava automation tool deployed as an Azure Functions custom handler. It processes Strava webhook events and automatically updates activities (titles, gear, descriptions, weather data). Built with Go's standard library `net/http` — no router frameworks.

Module: `github.com/lildude/strautomagically`

## Code Style

Write idiomatic Go. Follow [Effective Go](https://go.dev/doc/effective-go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) wiki.

- **Error handling**: Wrap errors with context using `fmt.Errorf("doing thing: %w", err)`. Prefer `errors.Is` / `errors.As` for checking. Don't discard errors silently unless the reason is documented.
- **Naming**: Short, clear names. Receivers are 1–2 letter abbreviations of the type. Avoid stuttering (`cache.Cache` not `cache.CacheService`). Acronyms are all-caps (`URL`, `ID`, `HTTP`).
- **Interfaces**: Define interfaces at the point of consumption, not the point of implementation. Accept interfaces, return concrete types.
- **Structs**: Use functional options or plain constructors; avoid `init()` for configuration. Read environment variables at startup and pass config explicitly.
- **Context**: Thread `context.Context` as the first parameter through all I/O operations. Use it for cancellation and timeouts on HTTP calls and Redis operations.
- **Logging**: Use `log/slog` for structured logging instead of `log.Println` with manual prefixes.
- **Packages**: Keep packages focused on a single responsibility. Don't create `util` or `common` packages — find a more descriptive name or inline the code.

## Architecture

```
cmd/strautomagically/main.go    — Entry point, route registration, server setup
internal/
├── cache/                      — Redis cache abstraction (interface + implementation)
├── calendarevent/              — TrainerRoad ICS calendar fetch & parse
├── client/                     — Generic REST API client
├── handlers/
│   ├── auth/                   — OAuth2 flow + webhook subscription management
│   ├── callback/               — Webhook challenge-response validation
│   └── update/                 — Core business logic: activity rules engine
├── strava/                     — Strava API types and methods
└── weather/                    — OpenWeatherMap weather + AQI integration
```

All domain code lives under `internal/`. The `cmd/` package is the entry point only — keep it thin.

When adding new functionality, prefer dependency injection over creating clients and caches inline in handlers. Handler functions should receive their dependencies (cache, API clients, config) rather than constructing them.

## Build and Test

| Command | Purpose |
|---------|---------|
| `make build` | Build the binary |
| `make test` | Run all tests (`ENV=test go test -p 8 ./...`) |
| `make coverage` | Run tests with coverage report |
| `make lint` | Run `golangci-lint run` |
| `make start` | Build and start locally with Azure Functions Core Tools |

Always run `make lint` and `make test` before considering work complete.

## Testing Conventions

- Use the standard `testing` package only — no testify or other assertion libraries.
- Use table-driven tests with `t.Run` subtests for all non-trivial functions.
- Use `t.Helper()` on all test helper functions.
- Use `t.Setenv()` for environment variable overrides.
- Place test fixture files in a `testdata/` directory alongside the test file.
- Use `httptest.NewServer` for HTTP server mocks and `miniredis` for Redis.
- Use `httpmock` for transport-level HTTP mocking when testing handlers that call multiple external APIs.
- Silence log output in tests with `log.SetOutput(io.Discard)`.
- Compare structs with `reflect.DeepEqual`; use direct `got != want` for scalars.

## Conventions

- **Configuration**: All config comes from environment variables. Use `os.Getenv` / `os.LookupEnv`. No config files or Viper.
- **Templates**: Go `text/template` files live in `templates/`. Resolve the path from the binary's working directory.
- **Azure Functions**: The binary is a plain HTTP server. Azure routes requests via `host.json` and `*Function/function.json` definitions. Don't import Azure-specific SDKs.
- **External APIs**: Strava, OpenWeatherMap, and TrainerRoad are the three external integrations. Use the generic `client.Client` for REST calls. API keys go in env vars, never in code.
- **Caching**: The `Cache` interface in `internal/cache/` abstracts Redis. Use `SetJSON`/`GetJSON` for structured data. Don't add TTLs unless explicitly needed.
- **Deduplication**: The last processed activity ID is cached to prevent duplicate webhook processing.
