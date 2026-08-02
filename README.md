# GymTracker AI

GymTracker AI is a planned web application for building strength-training programs, recording workouts and body measurements, visualizing progress, generating weekly reports, and consulting an AI coach grounded in the authenticated user's own training data.

The AI coach is designed around a strict safety boundary: apart from the text explicitly submitted in the owned coach conversation, OpenAI receives user-specific facts only as minimized results from allowlisted Go backend tools. The model cannot directly edit a training program. It can create a validated proposal, show the user a clear diff and rationale, and the backend applies it only after explicit confirmation.

## Planned capabilities

- Create, activate, and archive multi-day training programs.
- Track exercises, sets, weight, repetitions, and RIR during a workout.
- Browse snapshot-based workout history, changeable only through an explicit reopen/correction flow, and personal records.
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
- [Implementation plan](docs/implementation-plan.md)
- [Security model](docs/security.md)
- [AI coach design](docs/ai-coach.md)

Repository-wide Codex rules are defined in [AGENTS.md](AGENTS.md).

## Implemented foundation

The monorepo now contains a working technical foundation:

- a Go 1.22.2-compatible API using chi, environment configuration, structured JSON logging, request IDs, panic recovery, request logging, graceful shutdown, and unified problem responses;
- `GET /health/live` and `GET /health/ready` operational endpoints;
- a minimal Next.js App Router application with TypeScript, Tailwind CSS, and persisted light/dark theme selection;
- reproducible Go and npm lock files, unit tests, Dockerfiles, Compose configuration, Make targets, and foundation CI.

Authorization, database access/migrations, exercises, programs, workouts, reports, progress features, and AI Coach are intentionally not implemented yet. The PostgreSQL Compose service is prepared, but backend readiness currently reflects only application state.

## Local development

Requirements: Go 1.22.2, Node.js 22, npm 10, Docker with the Compose plugin for container-based startup.

Create an untracked local environment file before using Compose:

```bash
cp .env.example .env
```

Run backend and frontend in separate terminals:

```bash
make backend-run
make frontend-dev
```

The frontend is available at `http://localhost:3000`; the backend listens at `http://localhost:8080`. Health checks:

```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

## Make commands

- `make backend-run` — run the API locally.
- `make backend-test` — run Go tests.
- `make backend-fmt` — format Go files.
- `make frontend-dev` — run the Next.js development server.
- `make frontend-test` — run frontend unit tests.
- `make frontend-build` — create a production frontend build.
- `make compose-up` — build and start PostgreSQL, backend, and frontend.
- `make compose-down` — stop the Compose stack without deleting its database volume.

Docker and the Compose plugin are installed in the current development environment, but the current user may not be able to access the Docker daemon. Do not use `sudo` or alter host permissions; `docker compose --env-file .env.example config --quiet` can still validate the Compose file statically.
