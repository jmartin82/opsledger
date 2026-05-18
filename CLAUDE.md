# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Full-stack change tracking system: React/TypeScript frontend + Go/Echo backend + MariaDB. Tracks infrastructure, deployment, and configuration changes with three auth methods (JWT, API keys, MCP).

## Commands

### Docker (full stack)
```bash
make up              # Start all services (UI :8080, backend :8081, DB :3306)
make up-build        # Start with rebuild
make down            # Stop all services
make db-cli          # MariaDB shell (user: tracker, pass: tracker_dev, db: ops_ledger)
make db-reset        # Destroy DB volume and recreate
make logs-backend    # Tail backend logs
```

### Backend development
```bash
cd backend && go run .                        # Start backend (port 8081)
make test                                      # Unit tests only
make test-v                                    # Unit tests verbose
make test-integration                          # Integration tests (requires running MariaDB)
make test-all                                  # All tests
cd backend && go test ./handlers/ -run TestChangeCreate -v   # Single test
```

### Frontend development
```bash
cd frontend && npm run dev          # Vite dev server (:5173)
cd frontend && npm run build        # Production build
cd frontend && npm run lint         # ESLint
cd frontend && npm run test         # Vitest
```

## Architecture

### Backend (Go/Echo)

All handlers receive `*sql.DB` directly — no ORM, no repository layer. Models contain both struct definitions and SQL query functions (e.g., `models.CreateChange()`, `models.ListChanges()`).

**Auth flow**: Two middleware functions in `middleware/`:
- `JWTAuth` — extracts JWT claims, sets `c.Set("claims", claims)` on the Echo context
- `APIKeyOrJWT` — checks if Bearer token has `ol_live_` prefix for API key path, otherwise falls back to JWT. API key path sets `c.Set("apiKeyScopes", ...)` and `c.Set("apiKeyCreatedBy", ...)`

Handlers check for auth type by testing which context values are present (claims vs apiKeyScopes).

**MCP endpoint** (`POST /mcp`): Unauthenticated JSON-RPC 2.0 handler — does not go through auth middleware. Implements `initialize`, `tools/list`, and `tools/call` methods.

**Database migrations**: `database/migrations.go` uses `CREATE TABLE IF NOT EXISTS` — no migration versioning system. Three tables: `users`, `api_keys`, `changes`. Additive schema changes go in the ALTER TABLE block that runs after the CREATE TABLE statements; use `isMySQLDupErr()` to swallow duplicate-column/index errors (MySQL 1060/1061) so migrations stay idempotent.

### Frontend (React/TypeScript)

- `src/lib/api.ts` — thin fetch wrapper; auto-clears token and redirects to `/login` on 401
- `src/contexts/AuthContext.tsx` — central auth state; login/register/logout are wired to backend, but user management (roles, status) and audit log still use localStorage mocks (marked with TODO comments)
- `src/components/RequireAuth.tsx` — route guard with `minRole` prop for role-based access
- UI components in `src/components/ui/` are shadcn/ui (don't modify these directly)

### Testing patterns (backend)

Unit tests use `go-sqlmock` to mock `*sql.DB`. Each handler test file has a `setup*Context` helper that creates an Echo context with the right auth (JWT claims or API key scopes). Integration tests use `//go:build integration` tag and require a running MariaDB instance.

Test naming convention: unit tests start with `Test`, integration tests start with `TestIntegration`. The Makefile `test` target uses `-run "^Test[^I]"` to exclude integration tests.

## Environment Variables (backend)

`PORT`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `JWT_SECRET` — all have dev defaults in `config/config.go` so the backend runs without a .env file against local MariaDB.

## Change status model

The `changes` table has two fields that work together:

- `event_at` — authoritative "when": execution time for executed changes, planned date for scheduled ones. Exposed as `"timestamp"` in the JSON API for backwards compatibility.
- `status` — `'executed'` (default) or `'scheduled'`. **Overdue is not stored** — it is computed wherever needed as `status = 'scheduled' AND event_at < NOW()`. The `?status=overdue` query filter and the `passesFilters` SSE helper in the frontend both implement this same logic.

`models.ErrAlreadyExecuted` is a sentinel error returned by `ConfirmChange` when the change is already executed; the handler maps it to HTTP 409.

When adding new columns to `changes`, update the shared `changeColumns` slice and `addChangeRow` helper in `handlers/changes_test.go` — all mock SELECT rows must stay in sync with the actual column order.

## Roles

`admin` > `editor` > `viewer`. Viewers can only read changes. Editors can read and create changes. Admins can do everything including API key management.
