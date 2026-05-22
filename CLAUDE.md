# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Memory Requirements

**IMPORTANT**: All communication and responses must be in Chinese (中文). Claude must use Chinese for all interactions with the user, including:
- 所有回复必须使用中文
- 代码注释和文档说明使用中文
- 错误信息和提示使用中文
- 任何交流都必须用中文进行

## Common Development Commands

### Building the Project
- `make build` - Build both frontend and backend
- `make build-frontend` - Build frontend only (Vue.js)
- `make build-backend` - Build backend only (Go binary)
- `make build-multi-arch` - Build for multiple architectures (Linux, Windows, macOS)

### Development
- `make dev` - Start development server (backend only)
- `go run ./cmd/main.go` - Direct backend development
- Frontend development: `cd fe && corepack pnpm@9.15.9 dev` (starts Vite dev server on port 5173)

### Testing and Quality
- `make test` - Run Go tests
- `make lint` - Run golangci-lint
- Frontend linting: `cd fe && corepack pnpm@9.15.9 lint` (ESLint + stylelint)
- Frontend formatting: `cd fe && corepack pnpm@9.15.9 format` (Prettier)

### Docker
- `make docker-build` - Build Docker image
- `make docker-run` - Run Docker container
- `make docker-stop` - Stop Docker container

### Documentation
- `make swag-init` - Generate Swagger documentation
- `make swag-fmt` - Format Swagger comments

## Architecture Overview

This is a Go-based web application that provides WebDAV access to Cloud189 (天翼云盘) shared files with a Vue.js frontend.

### Backend Architecture (Go)

**Entry Point**: `cmd/main.go` - Main application entry

**Core Structure**:
- `internal/` - All internal packages
  - `bootstrap/` - Application bootstrap and service context
  - `configs/` - Configuration management
  - `framework/` - Custom framework components (HTTP context, task context)
  - `handler/` - Request handlers
    - `http/` - HTTP API handlers
    - `consumer/` - Message consumers for background tasks
  - `models/` - Data models and database entities
  - `router/` - Route definitions
  - `services/` - Business logic layer
  - `types/` - Type definitions and constants

**Key Services**:
- `cloudtoken` - Cloud189 authentication token management
- `cloudbridge` - Cloud189 API integration
- `virtualfile` - Virtual file system for Cloud189 shares
- `mediafile` - Media file STRM generation
- `mountpoint` - Storage mount point management
- `user`/`usergroup` - User and group management
- `autoingestplan` - Automated file ingestion scheduling

**Framework**: Built on Gin with custom framework abstractions for HTTP and task contexts.

### Frontend Architecture (Vue.js)

**Location**: `fe/` directory

**Tech Stack**:
- Vue 3 + TypeScript
- Vite build tool
- Pinia state management
- Naive UI component library
- Vue Router for navigation

**Structure**:
- `src/api/` - API client functions
- `src/components/` - Reusable Vue components
- `src/stores/` - Pinia stores
- `src/views/` - Page components
- `src/utils/` - Utility functions

### Database

- SQLite (via GORM) as primary database
- Supports MySQL and PostgreSQL as alternatives
- Database file location: `data/share.db`

### Key Features

1. **WebDAV Interface** - Provides standard WebDAV access to Cloud189 files
2. **Virtual File System** - Maps Cloud189 share links to directory structure
3. **Media STRM Generation** - Creates STRM files for media server compatibility
4. **Multi-tenant Architecture** - Supports users and groups with permissions
5. **Background Processing** - Automated file ingestion and sync tasks
6. **Media Streaming** - Built-in media player with HLS support

### Configuration

Configuration files in `etc/` directory:
- Main config: `etc/config.yaml`
- Database file: `data/share.db`
- Media directory: `media_dir/`
- Logs: `logs/share.log`

### Testing

- Go tests use standard testing package
- Test files follow `*_test.go` pattern
- Example: `internal/services/cloudbridge/fetch_files_test.go`
- Run service tests: `go test ./internal/services/tmdb/... -v`
- Run all tests: `go test ./...`

### Unit Tests (New Features)

### API Endpoints

- `/api/user/*` - User management
- `/api/cloudtoken/*` - Token management
- `/api/storage/*` - Storage management
- `/api/setting/*` - System settings
- `/api/media/*` - Media management
- `/api/telegram/*` - Telegram Bot management
- `/dav/*` - WebDAV interface

### New Features (v2.0+)

#### Telegram Bot Support
- Bot commands: `/start`, `/help`
- Auto-recognize 189 share links
- HTTP/HTTPS/SOCKS5 proxy support (env: `TG_PROXY`)
- Configuration: `/api/telegram/setting`, `/api/telegram/test`, `/api/telegram/users`

#### TMDB + Douban Subscription System
- Daily hot resources from TMDB (movie/popular, tv/popular)
- Douban hot movie subscription
- Keyword subscription support
- AI-powered auto-upgrade via OpenAI (search higher quality resources)
- Pan search integration (default: https://tg.252035.xyz)

#### OpenAI Integration
- API key from env: `OPENAI_API_KEY`
- Base URL configurable (default: https://api.openai.com)
- Model configurable (default: gpt-4o-mini)
- Used for generating optimized search keywords for quality upgrades

#### Database Models (New)
- `telegram_settings` - Telegram Bot configuration
- `telegram_users` - Telegram user management
- `subscriptions` - Subscription management
- `match_history` - Resource matching history
- `daily_hot_history` - Daily hot resource processing history

### Development Notes

- The project uses Go 1.25+ with modern Go features
- Frontend uses modern ES6+ and TypeScript
- Docker support for easy deployment
- Multi-architecture builds available
- Swagger documentation generation for APIs
