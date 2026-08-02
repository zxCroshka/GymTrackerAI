# GymTracker AI

GymTracker AI is a web application under active development for building strength-training programs, recording workouts and body measurements, visualizing progress, generating weekly reports, and consulting an AI coach grounded in the authenticated user's own training data.

The AI coach is designed around a strict safety boundary: apart from the text explicitly submitted in the owned coach conversation, OpenAI receives user-specific facts only as minimized results from allowlisted Go backend tools. The model cannot directly edit a training program. It can create a validated proposal, show the user a clear diff and rationale, and the backend applies it only after explicit confirmation.

## Planned capabilities

- Create, activate, and archive multi-day training programs.
- Track exercises, sets, weight, repetitions, and RIR during a workout.
- Browse snapshot-based workout history and make explicit, owner-authorized corrections to completed workouts without reopening them.
- Record body mass, body measurements, and daily wellness.
- Visualize training volume, exercise performance, and body trends.
- Produce deterministic weekly metrics with optional AI-written insights.
- Chat with an AI coach through safe, auditable backend tools.

## Agreed stack

The backend will be a Go 1.22-compatible modular monolith exposing a versioned REST API with `chi`, PostgreSQL, `pgx`, `golang-migrate`, JWT access and refresh tokens, OpenAPI, structured JSON logs, and unit/integration tests.

The frontend will use Next.js, TypeScript, App Router, npm, Tailwind CSS, accessible shadcn/ui-compatible components, TanStack Query, React Hook Form, Zod, and Recharts.

Local and CI infrastructure will use Docker, `docker compose`, a Makefile, GitHub Actions, and a secret-free `.env.example`.

## Architecture documentation

- [Product requirements](docs/product-requirements.md)
- [System architecture](docs/architecture.md)
- [Database schema](docs/database-schema.md)
- [REST API contract](docs/api-contract.md)
- [OpenAPI 3.1 contract](docs/openapi.yaml)
- [Implementation plan](docs/implementation-plan.md)
- [Security model](docs/security.md)
- [AI coach design](docs/ai-coach.md)

Repository-wide Codex rules are defined in [AGENTS.md](AGENTS.md).

## Implemented foundation

The monorepo now contains a working technical foundation:

- a Go 1.22.2-compatible API using chi, environment configuration, structured JSON logging, request IDs, panic recovery, request logging, graceful shutdown, and unified problem responses;
- a pgxpool PostgreSQL connection with bounded connect/ping timeouts, configurable pool limits, UTC sessions, startup ping, clean shutdown, and database-aware `GET /health/ready`;
- complete numbered golang-migrate forward/rollback SQL for the agreed normalized schema, plus a separate one-shot Compose migration service;
- production auth/profile foundations: Argon2id passwords, short-lived access JWTs, hashed rotating refresh JWT cookies with replay invalidation, protected routes, strict profile update/import, and metadata-only security audit for refresh replay;
- an idempotent seed command with 19 deterministic reviewed system exercises and a concrete pgx transaction helper;
- authenticated exercise catalogue routes for system/owned exercises, literal search, controlled filters, capability filters, cursor pagination, custom creation/update, and history-safe archive;
- authenticated full-aggregate program routes for create/read/update/archive, duplication, and transactional activation with one active program per user;
- authenticated owner-scoped workout routes for starting an ad-hoc workout or one snapshotted from an active program day, retrieving the single active workout, history/detail reads, metadata updates, nested exercise/set CRUD, previous exercise results, state-idempotent completion, CSV export, and explicit hard deletion with cascading children;
- direct completed-workout corrections guarded by ownership, the workout ETag, and one transaction while retaining completed status and a valid non-null completion instant; workout volume and Epley estimated 1RM for sets of at most 15 repetitions are currently calculated dynamically from source sets rather than stored as progress projections;
- authenticated measurement and wellness routes for canonical body values, sleep, quality/energy, steps, daily aggregate calories/macronutrients, strict local-civil-day uniqueness, and report staleness;
- deterministic progress dashboard, weight/moving-average series, per-exercise dynamics, and auditable personal records for max weight, repetitions at an exact weight, set volume, and Epley estimated 1RM;
- synchronous deterministic weekly report generation under a shared per-user source lock, with profile-local UTC week bounds, previous-week comparison, weight/wellness signals, new records, pain messages, immutable revisions/cutoffs, and transactional staleness/regeneration;
- `GET /health/live` and `GET /health/ready` operational endpoints;
- a minimal Next.js App Router application with TypeScript, Tailwind CSS, and persisted light/dark theme selection;
- reproducible Go and npm lock files, unit tests, Dockerfiles, Compose configuration, Make targets, and foundation CI.

Workout/measurement/progress/report frontend screens and AI Coach use cases remain intentionally unimplemented. This stage adds backend APIs only and makes completed-workout completion/correction/deletion rebuild personal records and stale affected current reports through narrow same-transaction ports. Weekly reports contain no AI output and make no OpenAI calls.

## Local development

Requirements: Go 1.22.2, Node.js 22, npm 10, Docker with the Compose plugin for container-based startup.

Create an untracked local environment file before using Compose:

```bash
cp .env.example .env
```

The simplest complete startup uses Compose. It waits for PostgreSQL health, runs migrations once in the dedicated `migrate` container, then starts the API and frontend:

```bash
make compose-up
```

To run Go commands directly on the host, export the untracked environment and make sure `DATABASE_URL` points to an already reachable PostgreSQL instance. The API now fails fast when the URL is missing or startup ping fails:

```bash
set -a
source .env
set +a
make backend-run
```

Run the frontend separately if needed:

```bash
make frontend-dev
```

The frontend is available at `http://localhost:3000`; the backend listens at `http://localhost:8080`. Health checks:

```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

Implemented API route groups are auth/profile, exercises, programs, workouts, body measurements (`/api/v1/measurements`), wellness (`/api/v1/wellness`), progress dashboard/weight/exercise/records, and deterministic weekly reports (`POST /api/v1/reports/weekly`, `GET /api/v1/reports[/{id}]`). Exact schemas, filters, archive rules, calculations, and ETag preconditions are in `docs/openapi.yaml` and `docs/api-contract.md`.

Registration returns an access token in JSON and sets the raw refresh token only as an HttpOnly cookie:

```bash
curl -i -c /tmp/gymtracker-cookie.txt \
  -H 'Content-Type: application/json' \
  -d '{"email":"athlete@example.com","password":"long-local-password"}' \
  http://localhost:8080/api/v1/auth/register
```

Refresh/logout requests must include an origin from `AUTH_ALLOWED_ORIGINS`. Production must use a Secure cookie, HTTPS origins, and independently generated JWT secrets.

## Make commands

- `make backend-run` — run the API locally.
- `make backend-test` — run Go tests.
- `make backend-integration-test` — run migration and all backend integration tests, including measurement/progress/report flows, against the dedicated disposable PostgreSQL database in `TEST_DATABASE_URL`; tests perform full up/down.
- `make backend-fmt` — format Go files.
- `make frontend-dev` — run the Next.js development server.
- `make frontend-test` — run frontend unit tests.
- `make frontend-build` — create a production frontend build.
- `make compose-up` — build and start PostgreSQL, backend, and frontend.
- `make compose-down` — stop the Compose stack without deleting its database volume.
- `make migrate-up` — run all pending migrations in a one-shot container.
- `make migrate-down` — roll back the most recent migration in a one-shot container.
- `make migrate-create name=add_example` — create a UTC-versioned up/down migration pair initialized with UTC session setup.
- `make db-seed` — idempotently install/update the baseline system exercise catalogue.

Schema migration is never performed by API startup. The Compose `backend` service depends on successful completion of the one-shot `migrate` service, so multiple API replicas cannot race to migrate the database.

PostgreSQL integration tests require a dedicated disposable database because they perform full forward and rollback migrations. They intentionally fail, rather than skip, when run with the `integration` tag without `TEST_DATABASE_URL`. GitHub Actions provisions `gymtracker_test` and runs the full tagged backend suite with PostgreSQL 16.

Docker and the Compose plugin are installed in the current development environment, but daemon availability can vary by session. Try Docker-backed integration checks when relevant; never use `sudo` or alter host permissions. If daemon access actually fails, `docker compose --env-file .env.example config --quiet` still validates Compose statically and the blocked container checks must be reported explicitly.
