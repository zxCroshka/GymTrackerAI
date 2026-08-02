# GymTracker AI implementation plan

Status: backend slices through deterministic measurement/progress/report implemented; frontend product slices and AI sequenced

Last updated: 2026-08-02

## 1. Delivery strategy

Implement vertical, working slices in dependency order. Each phase includes database constraints, Go use cases/routes/OpenAPI, Next.js UI where user-visible, and relevant tests. Do not create empty architecture folders, fake successful handlers, placeholder business logic, hidden TODOs, or speculative abstractions.

The modular-monolith boundaries in `architecture.md`, schema in `database-schema.md`, HTTP behavior in `api-contract.md`, and security/coach invariants are the baseline. If implementation evidence requires a change, update the affected documents with alternatives, impact, and migration rationale before changing code.

## 2. Environment constraints

- Backend must compile and test with installed Go `1.22.2`. Check dependency `go` directives/release notes before selection or upgrade; never let tooling silently require a newer Go version.
- Frontend uses installed Node.js 22, npm 10, and an npm lockfile. Do not use pnpm, Yarn, or Bun.
- Use `docker compose`, not `docker-compose`.
- Docker CLI/Compose are installed. Daemon availability is session-dependent: always try Docker-backed integration checks, never use `sudo` or modify host permissions, and report an actual access failure explicitly rather than assuming one.
- Foundation dependencies are locked in `backend/go.sum` and `frontend/package-lock.json`, including pgx v5.7.2 and golang-migrate v4.18.2 with compatible Go directives.
- At the user's explicit request, the complete agreed schema was migrated before business-module implementation. This changes sequencing, not module ownership: no empty business layers or handlers were introduced, and each later vertical slice must test and use its owned existing tables. Future schema changes use incremental migrations.

## 3. Phase 0 — design baseline (completed)

Deliverables:

- product requirements and core journeys;
- modular-monolith architecture and dependency rules;
- normalized PostgreSQL logical schema;
- `/api/v1` prose contract and security conventions;
- AI tool/approval flow and threat controls;
- implementation sequence and repository rules.

Historical exit gate at completion of Phase 0:

- all required documents exist;
- module/table/route/state/time/unit names agree;
- design explicitly states unresolved product choices;
- repository still had no backend/frontend source, dependency manifests, migrations, or infrastructure implementation.

## 4. Phase 1 — executable foundation and contracts

Implementation was authorized by subsequent requests. The slices provide the Go/chi server and middleware, PostgreSQL startup/readiness foundation, full initial schema and rollback, one-shot migration and seed commands, minimal Next.js/Tailwind/theme shell, lock files, Dockerfiles, Compose, Makefile, CI, and an OpenAPI 3.1 source for implemented auth/profile routes. Frontend API/auth shells remain later work and are not represented by placeholders.

Deliverables:

- agreed repository layout introduced with functioning minimal components, not empty placeholders;
- Go module compatible with 1.22.2, chi server, configuration validation, graceful shutdown, UTC pgx pool, request IDs, structured JSON logging/redaction, problem responses, and liveness/readiness endpoints;
- initial OpenAPI 3.1 document with common envelope/problem/pagination/security/header schemas;
- Next.js TypeScript App Router setup using npm, Tailwind, accessible UI primitives, lint/type-check/build, API client boundary, and auth/app shells;
- `compose.yaml` for web/api/PostgreSQL, Dockerfiles, `.env.example` with placeholders only, `.gitignore`, Makefile, and GitHub Actions;
- explicit one-shot migration command/process and complete initial agreed schema, delivered ahead of owning use cases by explicit request;
- CI caching and least-privilege permissions.

Verification:

- Go format/vet/unit tests/build on Go 1.22.2;
- npm clean install/lint/type-check/test/build on Node 22;
- OpenAPI lint and problem/envelope contract tests;
- Compose config validation that does not require daemon where possible;
- container build/start/health checks once daemon access exists;
- secret/redaction checks.

Exit gate: a real health/readiness slice runs, fails fast on bad configuration, and has no fake domain endpoints.

## 5. Phase 2 — persistence, auth, and profile

Current completion: the requested backend subset is implemented—registration/login, access protection, refresh rotation/replay invalidation, logout/family revocation, own-profile GET/PATCH, and strict transactional JSON import with optional initial measurement. The incremental `000002` migration and OpenAPI/prose contracts are synchronized. Session administration, password change, AI settings, deletion, and all frontend forms remain deliberately future work.

Deliverables:

- auth/profile persistence and queries against the existing foundation tables, with incremental migrations only for reviewed schema changes;
- Argon2id password hashing, JWT claim/algorithm/purpose validation, separate access/refresh keys, refresh cookie/rotation/reuse-family revocation, auth version, logout/session management;
- registration transaction creating default profile; authenticated profile read/update with ETag; versioned AI enablement/notice/disable cancellation settings; password-confirmed account soft deletion/session revocation;
- exact CORS/origin/cookie/security-header policy;
- Next.js login/register/session bootstrap, in-memory access token behavior, accessible profile form and conflict handling;
- synchronized OpenAPI operations/schemas/examples.

Verification:

- unit tests for password/JWT/claims/domain validation;
- PostgreSQL integration tests for registration uniqueness/transaction rollback, token rotation/reuse/concurrency/revocation, profile tenant isolation and version conflict;
- HTTP contract tests for cookie attributes, no-store, origin/CORS, status/problems and unknown fields;
- frontend form/session expiry/reload tests without persisting access token;
- logs contain no password/token/cookie/key.

Exit gate: two fixture users cannot discover or mutate one another; all documented session flows work under retry/concurrency.

## 6. Phase 3 — exercise catalogue and programs

Current completion: the requested backend slice is implemented. It includes the tenant-safe searchable system/custom catalogue, 19-entry idempotent system seed, archive semantics, full program aggregate create/read/update/archive/duplicate/activate, contiguous ordering validation, ETags, transactional tree replacement, transactional single-active switching, OpenAPI/docs, and unit/PostgreSQL integration tests. Program/exercise frontend screens and custom-exercise restore remain future work.

Deliverables:

- exercise/program persistence against existing checked, tenant-safe foundation tables;
- seed strategy for reviewed global exercises (deterministic data, not migration-time network calls);
- search/filter and owned custom-exercise archive; restore is a later lifecycle addition;
- complete program aggregate CRUD, duplicate, ordering, version/ETag, active replacement, inactive/archive lifecycle;
- Next.js exercise picker/custom form, program list/editor/reordering and conflict UX;
- OpenAPI and accessible forms with canonical-value validation.

Verification:

- unit tests for prescription and lifecycle rules;
- integration tests for global/custom visibility, cross-user references, archive/history constraints, one-active-program invariant, concurrent reorder/edit/activate and root version increments;
- API/filter/cursor/ETag tests, with ambiguous duplicate retry behavior documented until durable replay is implemented;
- frontend keyboard ordering/accessibility, form and `412` refresh-flow tests.

Backend exit gate reached: a user can create and activate a valid multi-day program; application/database constraints reject cross-user or invalid prescriptions and retain superseded/history rows. The phase remains open only for its planned frontend UX.

## 7. Phase 4 — workout logging and history

Current completion: the requested backend slice is implemented. It provides owner-scoped start from an active program day or ad-hoc start, one persisted in-progress workout, history/detail/active reads, metadata updates, nested exercise/set CRUD, previous exercise results, state-idempotent completion, CSV export, and explicit hard deletion. Completed-workout completion/correction/deletion now invokes the implemented transactional personal-record rebuild and affected-report staleness ports.

Deliverables:

- workout persistence against the snapshot-, tenant-, capability-, and version-constrained tables, extended through an incremental migration;
- transactional start from a current day of the authenticated user's active program, including copied program/exercise snapshots, or ad-hoc start without a program source;
- owner-scoped list, active, aggregate detail, strict metadata PATCH, nested exercise/set CRUD, previous-result lookup, and bounded CSV export;
- at most one `in_progress` workout per user, with the database partial unique index as final authority;
- state-idempotent completion: retrying completion of the same owned completed aggregate returns its completed representation without changing completion time, version, or derived facts again;
- direct owner-only corrections of completed workout metadata and children, guarded by the root ETag and one transaction while preserving completed status and a valid non-null completion instant;
- dynamically calculated workout volume and calculation-versioned Epley estimated 1RM, with Epley excluded above 15 repetitions;
- explicit owner-authorized hard deletion of a workout and database-cascaded child rows; it is never an archive, background cleanup, or implicit lifecycle transition;
- immutable source snapshots under later program/exercise edits and exactly one root ETag increment per successful aggregate mutation;
- Next.js active-workout logging optimized for common set entry, history/detail, error/retry/offline-not-supported messaging;
- synchronized OpenAPI for all implemented lifecycle, nested, previous-result, and export operations.

The backend has concrete progress/report ports rather than fake hooks. Their failure rolls the workout mutation back, preserving source/projection consistency.

Verification:

- unit tests for lifecycle/state-idempotent completion, capability-aware set validation, score/RIR/unit/time rules, volume/Epley boundaries, and snapshot creation;
- integration tests for owner isolation, source visibility, snapshot immutability after program/exercise edits, one-in-progress races, concurrent aggregate changes, direct completed corrections, completion retries, and hard-delete cascades;
- HTTP pagination/filter/active/previous/export, ETag, CSV, and problem-response tests;
- frontend rapid-entry, duplicate click/retry, keyboard/mobile viewport and accessible status tests.

Backend exit gate reached: program-to-completed-workout-to-history is reliable, completion is safe to retry by state, direct completed corrections are transactional and versioned, deletion is an explicit owner action, and later program edits cannot alter the workout snapshot. The phase remains open for its planned frontend UX; progress projections and report invalidation are delivered in their owning later phases.

## 8. Phase 5 — measurements and progress

Current completion: the requested backend subset is implemented. It includes body measurement create/list/versioned correction/delete, daily wellness create/list with civil-day uniqueness, sleep/activity/daily aggregate nutrition, dashboard/weight/exercise/record queries, and transactionally rebuildable PR discoveries. Frontend forms and charts remain future work.

Deliverables:

- measurement/progress persistence against existing foundation tables;
- body measurement CRUD through the requested routes and wellness create/list with item versions, canonical units, backend-calculated UTC/IANA civil-day boundary and uniqueness;
- deterministic personal-record recalculation tied to workout completion, direct completed correction, and explicit deletion through a narrow transactional progress port;
- bounded progress summary, body, volume, exercise and record endpoints with calculation versions;
- persisted-projection fixtures for the documented calculation-versioned Epley formula and its 15-repetition eligibility boundary;
- Next.js measurement/wellness forms, progress filters, accessible Recharts plus text/table equivalents.

Verification:

- numeric conversion/rounding/formula/DST unit tests;
- PostgreSQL integration tests for wide-measurement checks, daily-boundary uniqueness, tenant isolation, PR source/audit/recalculation and no public PR writes;
- aggregate series fixtures for empty/missing data, half-open ranges/granularity and query bounds;
- frontend metric/unit labels, empty states, chart accessibility and form conversion tests;
- query-plan review with representative synthetic history.

Backend exit gate reached: returned series/records are reproducible from source sets/measurements, bounded, correctly labelled and explicit when empty. The phase remains open only for frontend UX.

## 9. Phase 6 — weekly reports

Current completion: deterministic manual generation is implemented synchronously without OpenAI. It captures a stable logical snapshot in a `READ COMMITTED` transaction under the shared per-user source lock, inserts immutable ready revisions, returns an existing current ready artifact unchanged, regenerates stale current artifacts, and exposes owner-scoped list/detail routes. Queue/runner and AI narrative are deliberately not implemented.

Deliverables:

- report persistence against the existing revisioned `weekly_reports` table and source indexes;
- deterministic metric schema/version and UTC week-boundary calculation from profile IANA zone; implemented transport uses ready/stale states while persistence retains future lifecycle support;
- synchronous state-idempotent manual generation; internal runner claim/lease/retry behavior is deferred until asynchronous AI work is authorized;
- capture deterministic metrics/cutoff in one short transaction under the shared source lock, preserve old current artifact until a replacement is ready, and mark reports stale on every affecting workout completion/direct correction/deletion or measurement/wellness source mutation;
- future Next.js current/revision/status/metrics/source-link UI;
- AI insight field/status supported but disabled/not requested until coach infrastructure is production-ready.

Verification:

- unit/golden tests for metric schema/calculations and missing data;
- integration fixtures around DST (167/168/169-hour weeks), period/source cutoffs, one-current-revision, concurrent generate/regenerate, and tenant isolation; runner crash/lease coverage belongs to the future asynchronous AI phase;
- API synchronous create/existing-ready/stale-regeneration/status tests;
- frontend pending/failure/stale/revision/accessibility tests.

Backend exit gate reached: deterministic reports work without OpenAI and retain immutable revisions/cutoffs; frontend report UX remains future work.

## 10. Phase 7 — AI coach and report insight

Prerequisite: effective OpenAI project data controls, a versioned data-use notice and explicit AI enablement/disable behavior, model evaluation target, spend/rate budgets, incident contact, and server-side API key are approved. The exact legal basis is a launch-policy decision, not inferred by code. Do not put a real key in repository configuration or tests.

Deliverables:

- coach persistence against existing foundation tables, with the security invariants revalidated in module integration tests;
- server-only Responses API adapter with `store: false`, configured tested model/snapshot, safety identifier, timeouts and bounded retries; extract only a technical `platform/openai` HTTP/auth/redaction transport when report insight becomes its second concrete caller;
- static versioned developer policy, only bounded same-conversation direct text context, all profile/training/body/wellness/progress/report facts through safe backend tools, and moderation/safety response handling;
- strict sequential tool registry exactly as scoped in `ai-coach.md` using module public ports and minimized audit;
- durable asynchronous coach-message processing with reconstructed limited principal, current AI enablement/notice checks, attempt fencing, archive/disable/delete cancellation and retry metadata;
- typed canonical program proposal/hash/deduplication/expiry;
- separate synchronous confirm/reject routes and atomic program command transaction;
- optional weekly-report narrative produced only from stored deterministic metrics;
- Next.js conversation/polling/error UX and accessible exact-diff confirmation/rejection UI.

Verification:

- deterministic tool/schema/domain/tenant/range/result-size/audit tests with a fake local provider transport (not fake business behavior);
- integration tests proving tool loops and run-local proposal validation do not mutate program rows and successful response persistence creates only a proposed recommendation;
- confirm auth/owner/ETag/key/hash/state/expiry/version checks, concurrency/idempotency, fault rollback and stale-proposal tests;
- log/API/key/prompt/reasoning redaction tests and outbound contract tests requiring `store: false`, full transient Responses continuation items/call IDs, and no long-term provider reasoning persistence;
- versioned model evaluations and adversarial prompt-injection/medical/dangerous-training suites;
- explicit provider-failure tests proving core tracking and deterministic reports remain available;
- controlled staging smoke test using backend secret only, never CI forks/untrusted contexts.

Exit gate: evaluation thresholds are documented/met, privacy controls verified, spend alerts active, and automated proof shows no program mutation without explicit valid confirmation.

## 11. Phase 8 — hardening and release readiness

Deliverables:

- representative load/query-plan/pool/backpressure tuning;
- complete accessibility review of all critical journeys;
- dependency/container/action pinning, vulnerability/secret/container scanning and update process;
- TLS/proxy/CSP/CORS/cookie/secrets configuration review;
- encrypted backup/restore test, account purge/retention runbook, incident response and provider-key rotation drill;
- dashboards/alerts for HTTP/DB/jobs/auth/OpenAI budget/errors;
- OpenAPI compatibility check, operations/run/deployment documentation and release checklist.

Verification:

- full backend/frontend/unit/integration/contract/e2e/security/evaluation suites in a production-like environment;
- fresh database migration up/down/forward test and upgrade test from previous release;
- container non-root/health/shutdown/resource/failure testing;
- restore and key-rotation exercises;
- no unresolved fake implementations, hidden TODOs, skipped critical checks, leaked secrets, or undocumented architecture changes.

Exit gate: named owners accept remaining risks and all public claims match tested behavior.

## 12. Test matrix

| Layer | Fast/default checks | PostgreSQL/container checks | External/staging checks |
|---|---|---|---|
| Backend domain | Go unit tests, race where applicable, fuzz/validation | transaction/state/constraint/concurrency fixtures | none |
| REST/OpenAPI | handler/contract/problem/header tests | tenant/idempotency/ETag/filter behavior | HTTPS/proxy smoke |
| Frontend | lint, TypeScript, component/form/query tests, build | API e2e against local stack | browser accessibility/smoke |
| Migrations | static lint/naming | empty/up/down/upgrade and constraint tests | production-like lock review |
| Reports/progress | golden deterministic fixtures | source cutoff/DST/concurrency/query plans | none |
| Coach | strict tool/policy/provider-adapter/evaluation fixtures | tenant/audit/proposal/confirm/rollback | controlled OpenAI staging eval only |
| Infrastructure | Compose config, workflow lint, secret scan | build/start/health/shutdown/scan | backup/restore/rotation/alerts |

Tests requiring Docker must be reported as skipped due to daemon permissions until the environment owner grants access; local unit/lint/build/design checks still run.

## 13. CI progression

Start with required jobs for documentation/OpenAPI, Go, and frontend. Add migration/integration jobs using a GitHub Actions PostgreSQL service once migrations exist; they do not depend on the local daemon. Add container build/scan and e2e after both applications run. OpenAI live calls are never required on normal PRs and never run for forks; deterministic adapter/tool/evaluation fixtures are the default.

No workflow performs `git push`, automatic history changes, or secret-bearing artifact upload. Workflow permissions are minimal and third-party actions reviewed/pinned.

## 14. Definition of ready for a slice

A slice is ready when:

- relevant requirement IDs and user/error/lifecycle paths are known;
- schema/API/module ownership and UTC/unit behavior are reconciled;
- threat/authorization/idempotency/concurrency implications are identified;
- OpenAPI change and tests/fixtures are specified;
- dependencies have a concrete use and verified Go 1.22.2/Node 22 compatibility;
- unresolved choices that materially change behavior are taken back to the user rather than guessed.

## 15. Definition of done for a slice

- Working production path is complete end-to-end; no fake implementation or hidden TODO remains.
- Database constraints, Go domain/application logic, REST/OpenAPI, Zod/forms/UI and documentation agree.
- Relevant format, lint, type, unit, integration, contract, migration, accessibility and build checks pass; environment-blocked checks are explicitly disclosed.
- Tenant/auth/error/retry/idempotency/concurrency and sensitive-log tests exist for the slice.
- Secrets are absent; logs/errors are redacted; configuration placeholders are documented.
- No agreed architecture changed without documented rationale/impact.
- User-visible limitations and failure/empty/pending states are clear.

## 16. Key risks and mitigations

| Risk | Mitigation |
|---|---|
| Cross-user data leak | tenant-safe FKs, mandatory scoped APIs, IDOR matrix for REST and every coach tool |
| History corrupted by program edits or unsafe corrections | immutable source snapshots under later program/exercise edits; owner-only ETag-protected corrections plus concrete same-transaction projection rebuild/report staleness ports |
| Duplicate/lost concurrent changes | transactions, root versions/ETags, idempotency records, row locks |
| Time-zone/DST report errors | store UTC boundaries + zone snapshot, half-open intervals, 167/169-hour fixtures |
| Go ecosystem moves beyond 1.22 | dependency compatibility gate, pin versions, planned Go upgrade rather than implicit change |
| Docker unavailable locally | no sudo/permission mutation; run non-container checks and CI PostgreSQL integration; report local skip |
| OpenAI latency/outage/cost | asynchronous durable work, strict limits/budgets/retries, deterministic core/report degradation |
| Prompt injection or tool exfiltration | no general tools/owner args, strict+domain validation, minimal output, sequential bounds and adversarial evals |
| Autonomous/stale AI program edit | proposal-only tool, exact visible diff/hash, explicit REST confirmation, base version/lock/atomic apply |
| Sensitive telemetry | central redaction, no body/prompt logging, minimized private audit, access/retention controls |
