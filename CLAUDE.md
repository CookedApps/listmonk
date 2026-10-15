# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

listmonk is a self-hosted, high-performance newsletter and mailing list manager. A Go backend and a Vue 2 SPA admin frontend are compiled and bundled into a **single binary** (frontend assets embedded via `stuffbin`). PostgreSQL is the only data store. Licensed AGPLv3.

## This is a fork

Prealpha fork of upstream `knadh/listmonk`, based on the **v6.2.0** release. We do not send changes upstream.

- Fork delta: `git log v6.2.0..master`. Main fork feature: per-campaign subscriber segments (`campaigns.subscriber_query`, migration `v6.3.0`).
- The Vue SPA in `frontend/` is ours to change. Upstream's v7 frontend freeze does not apply.
- Sync only with upstream **releases** (remote `upstream`), never with unreleased upstream `master`.
- Build and deploy: see `deploy/README.md`.

## Commands

All commands are run from the repo root via the `Makefile` unless noted.

### Build
- `make build` - compile the Go backend to `./listmonk` (frontend not embedded).
- `make build-frontend` - build the Vue frontend and email-builder into `frontend/dist`.
- `make dist` - full production build: backend + frontend + `stuffbin` pack into one self-contained `./listmonk`.

### Run in development (two processes)
1. `make run` - backend dev server on `:9000`, serving frontend assets from `frontend/dist` on disk (no stuffbin repack needed).
2. `make run-frontend` - Vite dev server on `:8080` with hot reload. **Develop against http://localhost:8080**; it proxies API calls to `:9000`.

A first-time DB setup is required before the backend starts (see Database below).

### Docker dev stack (Postgres + Mailhog + Adminer + app)
- `make init-dev-docker` - one-time: build images and install the DB.
- `make dev-docker` - bring the stack up (app on `:8080`, Mailhog UI `:8025`, Adminer `:8070`).
- `make rm-dev-docker` - tear down including DB volumes.

### Test
- `make test` (= `go test ./...`) - run all Go tests.
- Single Go test: `go test ./internal/manager -run TestName` (point at the package dir, filter with `-run`).
- `make test` has no Go test files yet, so it is a compile check only.
- Cypress E2E (admin UI): from `frontend/`, `npx cypress run` (headless) or `npx cypress open`. Needs a `make dist` binary at `./listmonk` that is already running, and MailHog on `:8025`. Specs reset the DB with `./listmonk --install --yes`, so point `config.toml` at a throwaway DB. Override the port with `CYPRESS_baseUrl`/`CYPRESS_apiUrl`. Known baseline: stock v6.2.0 fails the same 13 of 92 tests (archive, campaigns, subscribers) as the fork. A regression is any failure outside that set.

### Lint
- Frontend: `cd frontend && yarn lint` (ESLint over `.js`/`.vue`; also runs automatically as `prebuild`).
- Backend: no `golangci-lint` config in-repo; rely on `gofmt`/`go vet`.

### Database
The binary manages its own schema:
- `./listmonk --install --yes` - fresh install (creates schema).
- `./listmonk --upgrade` - apply pending migrations to an existing DB.
- `./listmonk --install --idempotent --yes` - safe to re-run (used by the docker init).

Migrations are idempotent Go files in `internal/migrations/`, one per release (e.g. `v6.2.0.go` exposes `V6_2_0`), registered in `migList` in `cmd/upgrade.go`. They run on `--upgrade`, and the last applied version is stored in the `settings` row `migrations`.

### Versions
Go `1.26.1` (see `go.mod`). Frontend: Yarn `1.22.x` (use `corepack yarn` if `yarn` is not installed, or pass `YARN="corepack yarn"` to `make`). The dev Docker image uses Node 16; Node 22 also builds.

## Architecture

### Wiring (`cmd/`)
The `cmd/` package is both the entrypoint and the HTTP layer. `cmd/init.go` (package `init()` + `initX()` helpers) builds everything: loads config, opens the `sqlx` DB, mounts the `stuffbin` filesystem, runs migrations, parses `queries/*.sql`, and prepares all statements. `cmd/main.go` assembles the central `App` struct (holds `core`, `manager`, `messengers`, `auth`, `media`, `bounce`, `queries`, `db`, `i18n`, config, etc.) and registers routes in `initHTTPHandlers`. **Every HTTP handler is a method on `*App`** (e.g. `func (a *App) GetCampaign(c echo.Context) error`), spread across `cmd/campaigns.go`, `cmd/subscribers.go`, `cmd/lists.go`, `cmd/public.go`, `cmd/settings.go`, etc.

### HTTP (Echo v4)
Routing is grouped: unauthenticated (`/api/health`, `/api/config`, `/api/lang/:lang`), authenticated admin API (`/api/*`), the admin SPA (`/admin/*`), and public-facing routes (`/subscription/...`, `/archive/...`, `/webhooks/...`). Admin endpoints are wrapped by a permission middleware (`pm(handler, "subscribers:get_all", ...)`) that enforces RBAC via the `auth` package.

### Layers
- **`internal/core`** - the CRUD/business-logic layer over the DB. Methods take/return domain models and return `echo.HTTPError` directly. Handlers in `cmd/` mostly delegate to `core`.
- **`models/`** - domain types (`Campaign`, `Subscriber`, `List`, `Template`, `Message`, `Bounce`, `User`, `Settings`, ...) and `models/queries.go`, the `Queries` struct of `*sqlx.Stmt` fields tagged with `query:"..."` names.
- **`internal/manager`** - the campaign send engine. `Manager.Run()` pulls due campaigns and batches of subscribers from the DB, spins up a per-campaign `pipe` (`pipe.go`) that renders templates per subscriber, injects open/click tracking, and pushes `Message`s to messengers under rate/concurrency limits.
- **`internal/messenger`** - delivery behind a `Messenger` interface (`Push`/`Flush`/`Close`): `email/` (pooled SMTP via `smtppool`, multiple servers keyed by from-address) and `postback/` (HTTP POST webhook of the full payload).
- **`internal/bounce`** - async bounce processing: a POP3 `mailbox/` scanner plus provider `webhooks/` (SES, Azure, Sendgrid, Postmark, Forwardemail, Lettermint). Events flow through a queue and are recorded via a callback into `core`.
- **`internal/subimporter`** - bulk CSV subscriber import with dedup, optin, and progress logging.
- **`internal/auth`** - sessions (`simplesessions`, Postgres-backed), OIDC, BasicAuth API tokens, and RBAC.
- **`internal/media`** - upload `Store` interface with `filesystem/` and `s3/` backends.
- Other: `internal/i18n`, `internal/events` (SSE to the UI), `internal/buflog` (in-memory log ring for the UI), `internal/captcha` (Altcha/hCaptcha), `internal/notifs`.

### Database access
SQL is not written inline in Go. It lives in `queries/*.sql` (`subscribers.sql`, `campaigns.sql`, `lists.sql`, `bounces.sql`, etc.) annotated with `-- name: query-name` comments. `goyesql` parses them and `goyesqlx` scans each into the matching `query:"..."`-tagged `*sqlx.Stmt` field on `models.Queries`. **To add or change a query: edit the relevant `queries/*.sql` file and the field/tag in `models/queries.go`.** App-level settings persist as a single JSONB row in the `settings` table, marshaled to/from `models.Settings`.

### Config
Layered via `koanf`, lowest-to-highest precedence: `config.toml` (see `config.toml.sample`), then `LISTMONK_*` env vars, then settings stored in the DB. Static startup config maps to the `Config`/`UrlConfig` structs; runtime-editable settings live in `models.Settings`.

### Asset embedding
`make dist` runs `stuffbin` to pack `config.toml.sample`, `schema.sql`, `queries/`, `permissions.json`, `static/`, `frontend/dist` (served at `/admin`), and `i18n/` into the binary. In dev (`make run`), assets are read from disk instead (the `-X main.frontendDir=frontend/dist` ldflag).

### Frontend (`frontend/`)
Vue 2.7 + Buefy/Bulma, Vuex, Vue Router, Vue-i18n, built with Vite (base path `/admin`). The API client is in `frontend/src/api/`; an Axios interceptor converts snake_case responses to camelCase, drives the Vuex loading state, and surfaces error toasts. A separate `frontend/email-builder/` (React/TS) is built and copied into `frontend/public/static/email-builder`. i18n language packs are JSON in the repo-root `i18n/`.
