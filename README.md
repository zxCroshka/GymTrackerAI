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

## Current status

The repository is in the design phase. It contains architecture and product documentation only; backend source code, frontend source code, dependencies, database migrations, Docker files, Makefile, workflows, and environment templates have not been created yet.

Docker and the Compose plugin are installed in the current development environment, but the current user cannot access the Docker daemon. Do not attempt to change host permissions with `sudo`; integration tests that need containers must be run after daemon access is provided by the environment owner.
