# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**kiro2api** is an AI API proxy server that bridges Anthropic/OpenAI API formats with AWS CodeWhisperer. It provides token pooling, multi-account management, and a Vue 3 management UI.

- **Backend**: Go 1.24 + Gin
- **Frontend**: Vue 3 + TypeScript + Vite + TailwindCSS + Pinia
- **Database**: SQLite (modernc.org/sqlite)

## Critical: Running the Backend

**MUST run from `backend/` directory**. The database path is `./data/kiro2api.db` (relative). Running from project root creates an empty database, causing Token loading to fail.

```bash
cd backend
./kiro2api
```

## Build Commands

### Backend (Go 1.24)

```bash
cd backend

# Build
go build -o kiro2api ./cmd/kiro2api

# Run (from backend/ directory only!)
./kiro2api

# Test
go test ./...

# Vet
go vet ./...

# Debug mode
LOG_LEVEL=debug ./kiro2api
```

### Frontend

```bash
cd frontend

# Dev server
npm run dev

# Build (outputs to backend/static/)
npm run build

# Preview
npm run preview
```

## Architecture

```
Client Request
    ↓
Auth Middleware (API Key / Bearer Token)
    ↓
Rate Limiter
    ↓
Format Converter (Anthropic/OpenAI → CodeWhisperer)
    ↓
Token Manager (sequential selection with failover)
    ↓
AWS CodeWhisperer API
    ↓
Stream Parser (SSE events)
    ↓
Response Converter (CodeWhisperer → Anthropic/OpenAI)
    ↓
Client
```

## Key Backend Modules

| Module | File | Purpose |
|--------|------|---------|
| Entry | `cmd/kiro2api/main.go` | Bootstrap, loads .env, creates AuthService |
| Server | `internal/server/server.go` | HTTP routes, middleware, CORS |
| Token Mgmt | `internal/auth/token_manager.go` | Sequential selection, cooldown, exhaustion detection |
| Converter | `internal/converter/codewhisperer.go` | Format translation |
| Parser | `internal/parser/` | SSE stream parsing |
| Types | `internal/types/` | Request/response models (anthropic.go, openai.go, codewhisperer.go) |
| Config | `internal/config/` | Model mapping, dynamic settings |
| Handler | `internal/server/handler/` | HTTP request handlers |

## Token Management Strategy

- **Sequential selection**: Tokens used in config order, rotating through pool
- **Auto-failover**: 429/5xx errors trigger `MarkTokenFailed()`, token enters cooldown
- **Exhaustion detection**: Usage limits checked on refresh; exhausted tokens auto-moved to "exhausted" group
- **Banned detection**: 401 + "Bad credentials" triggers auto-move to "banned" group
- **Cooldown duration**: Configurable via `config.CooldownSec` env var

## Model Mapping

| Anthropic Model Name | CodeWhisperer Internal ID |
|----------------------|---------------------------|
| `claude-opus-4-5-20251101` | `claude-opus-4.5` |
| `claude-sonnet-4-5-20250929` | `CLAUDE_SONNET_4_5_20250929_V1_0` |
| `claude-sonnet-4-20250514` | `CLAUDE_SONNET_4_20250514_V1_0` |
| `claude-3-7-sonnet-20250219` | `CLAUDE_3_7_SONNET_20250219_V1_0` |
| `claude-haiku-4-5` | `auto` |

## Environment Variables

| Variable | Required | Default |
|----------|----------|---------|
| `KIRO_CLIENT_TOKEN` | Yes* | - |
| `KIRO_AUTH_TOKEN` | Yes | - |
| `PORT` | No | 8080 |
| `GIN_MODE` | No | release |
| `LOG_LEVEL` | No | info |
| `LOG_FORMAT` | No | json |
| `KIRO_DB_PATH` | No | `./data/kiro2api.db` |
| `MAX_TOOL_DESCRIPTION_LENGTH` | No | 10000 |
| `RATE_LIMIT_QPS` | No | 50 |
| `RATE_LIMIT_BURST` | No | 100 |

*Required unless `api_keys` are configured in database.

## API Endpoints

| Path | Method | Purpose |
|------|--------|---------|
| `/v1/models` | GET | List models |
| `/v1/messages` | POST | Anthropic API |
| `/v1/chat/completions` | POST | OpenAI API |
| `/v1/messages/count_tokens` | POST | Count tokens |
| `/:group/v1/messages` | POST | Group-specific Anthropic |
| `/:group/v1/chat/completions` | POST | Group-specific OpenAI |
| `/api/tokens` | GET/POST/DELETE/PATCH | Token management |
| `/api/groups` | GET/POST/PUT/DELETE | Group management |
| `/api/settings` | GET/POST | Settings |
| `/api/keys` | GET/POST/PATCH/DELETE | API key management |
| `/api/stats` | GET | Metrics |

## Database

- SQLite at `./data/kiro2api.db` (relative to `backend/`)
- Stores: tokens, groups, api_keys, settings
- Auto-created on first run
- Use `internal/config/` for schema/queries

## Frontend Architecture

- **Build output**: `backend/static/` (served by Gin)
- **State**: Pinia stores
- **Views**: Dashboard, Tokens, Groups, Settings, Keys, Login
- **Components**: Layout, Sidebar, Modal, Toast, StatusBadge, Icon
- **API client**: Direct fetch to `http://localhost:8080/api/*`
