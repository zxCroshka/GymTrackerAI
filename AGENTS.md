# AGENTS.md

## Scope

These rules apply to the entire GymTrackerAI repository. Read the documents in `docs/` before changing architecture, data contracts, security-sensitive behavior, or AI coach behavior.

## Safety and repository rules

- Never run `git push`.
- Never run destructive Git commands, including `git reset --hard`, `git clean -fd`, forced checkout of files, history rewriting, or force-push commands.
- Never use `sudo`.
- Never change Git configuration.
- Never commit secrets, real credentials, private keys, access tokens, refresh tokens, database passwords, or populated `.env` files.
- Keep secrets in environment variables or an approved secret manager. Commit only documented placeholders in `.env.example`.
- Never expose or send `OPENAI_API_KEY` to the frontend, browser, logs, API responses, tests, or fixtures. All OpenAI API calls must originate from the Go backend.
- Do not attempt to repair Docker permissions with `sudo`. If the Docker daemon is unavailable, report it and provide verification that does not require changing host permissions.

## Agreed architecture

- Use Go `1.22.2` compatibility for the backend. Check every dependency's minimum Go version before adding or upgrading it; do not raise the repository Go version implicitly.
- Keep the backend a modular monolith with the module boundaries documented in `docs/architecture.md`: `auth`, `user`, `exercise`, `program`, `workout`, `measurement`, `progress`, `report`, and `coach`.
- Do not split the application into microservices unless an architecture document is updated with measured evidence, migration consequences, and explicit user approval.
- Use REST under `/api/v1`, `chi`, PostgreSQL, `pgx`, `golang-migrate`, JWT access and refresh tokens, structured JSON logs, and OpenAPI as documented.
- Use Next.js, TypeScript, App Router, Tailwind CSS, accessible shadcn/ui-compatible components, TanStack Query, React Hook Form, Zod, and Recharts on the frontend.
- Use `npm`; do not introduce `pnpm`, Yarn, Bun, or their lockfiles.
- Use `docker compose`; do not use the legacy `docker-compose` command.
- Do not add infrastructure, layers, repositories, interfaces, factories, generic helpers, or abstractions without a concrete current use case.
- Do not bypass module boundaries with ad hoc SQL or imports. Cross-module work must use the small public ports described in `docs/architecture.md`.
- Do not change an agreed architectural decision without documenting the reason, alternatives, security/data/API impact, and migration plan in the relevant document (and an ADR when implementation begins).

## AI coach rules

- Apart from text the user explicitly submits in the current owned coach conversation and its bounded same-conversation history, the model may access user data only through allowlisted backend tools. Profile, program, workout, measurement, wellness, progress, and report facts must never be preloaded or fetched outside those tools. Never provide generic SQL, arbitrary HTTP, filesystem, secret-reading, or unrestricted database tools.
- Derive the acting `user_id` from verified authentication context; never trust a model-supplied or client-supplied owner identifier.
- Treat prompts and model output as untrusted input. Validate tool arguments and recommendation payloads against strict schemas and domain rules in the backend.
- AI tools must not directly mutate programs, workouts, measurements, profiles, or authentication data. `propose_program_change` is a pure run-local validator; persistence happens only with a successfully completed assistant response.
- An AI program-change candidate must be persisted only as a `proposed` recommendation and applied only through the documented confirmation endpoint after an explicit user action, ownership checks, version checks, and an auditable transaction.
- Asynchronous AI/report work must use attempt fencing and recheck active account, current AI enablement/notice, resource ownership/state, and cancellation before provider/tools and before saving a result.

## Implementation quality

- After every change, run the smallest relevant tests and checks; before handing off a completed task, run all relevant unit, integration, contract, lint, type-check, and build checks that the environment permits.
- If a relevant check cannot run (for example, Docker daemon access is unavailable), state exactly which check was skipped and why.
- Do not leave fake implementations, hard-coded successful responses, placeholder business logic, silent fallbacks, hidden TODOs, or untracked follow-up work. If work cannot be completed, expose the blocker clearly.
- Keep OpenAPI, request/response validation, database behavior, and implementation synchronized.
- Use UTC `timestamptz` instants in PostgreSQL and RFC 3339 UTC timestamps at API boundaries. Preserve the user's IANA time zone only for presentation and calendar-boundary calculations.
- Store weights and lengths canonically in kilograms and centimetres; convert only at input/output boundaries.
- Preserve immutable workout history when programs or exercise metadata change.
- Prefer parameterized queries, explicit transactions, deterministic tests, and clock/ID injection only where tests or domain behavior require them.
- Structured logs must be JSON and must redact passwords, JWTs, refresh tokens, cookies, API keys, authorization headers, raw coach prompts, and unrestricted tool payloads.

## Documentation precedence

When documents conflict, stop and reconcile them before implementation. Product behavior is defined in `docs/product-requirements.md`; component and module rules in `docs/architecture.md`; persistence in `docs/database-schema.md`; HTTP behavior in `docs/api-contract.md`; security invariants in `docs/security.md`; and AI orchestration in `docs/ai-coach.md`.
