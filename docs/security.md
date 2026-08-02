# GymTracker AI security design

Status: security baseline for implementation and review

Last updated: 2026-08-02

## 1. Security objectives

The system must protect credentials, private training/body/wellness data, coach conversations, and the OpenAI credential; prevent cross-user access; preserve workout and recommendation audit integrity; and make autonomous AI mutation structurally impossible.

This design is defense in depth, but it does not claim a compliance certification. Retention periods, legal basis/consent text, deployment region, and incident-response ownership must be decided before a public launch.

## 2. Assets and threat actors

Sensitive assets:

- password hashes, JWT signing keys, access/refresh JWTs, session metadata;
- email/profile identifiers;
- programs, workouts, performance history, body measurements, wellness and notes;
- coach messages, recommendation proposals/decisions, minimized tool audit;
- PostgreSQL credentials/backups and `OPENAI_API_KEY`;
- audit/log records that could correlate a person with health-adjacent information.

Threats include unauthenticated attackers, a malicious authenticated user attempting IDOR, stolen browser credentials, credential stuffing, XSS/CSRF, SQL injection, compromised dependencies or CI, accidental operator/log disclosure, prompt injection, malicious model/tool arguments, and race/retry behavior that could apply an action twice.

## 3. Non-negotiable invariants

1. The backend derives the actor only from a verified access JWT; client/model owner fields never establish authorization.
2. Every private query/mutation is scoped by `user_id`; foreign private resources appear absent.
3. Passwords, raw refresh tokens, JWT signing keys, database passwords, and OpenAI keys never enter logs or committed files.
4. OpenAI calls originate only in the Go backend. `OPENAI_API_KEY` never reaches Next.js runtime variables, bundles, HTML, API responses, browser storage, fixtures, or telemetry.
5. The model has no arbitrary SQL, HTTP, MCP, filesystem, shell, secret, or core-data write tool.
6. A model-generated proposal cannot mutate a program. Only the authenticated recommendation confirmation route can apply it after full revalidation/version checks.
7. External OpenAI calls are not made inside database transactions.
8. Completed workout facts cannot be edited without the explicit reopen/correction workflow and downstream recalculation/staleness handling.

A change that violates or weakens one of these invariants requires explicit documented approval and corresponding threat/test updates.

## 4. Authentication

### 4.1 Passwords

- Hash passwords with Argon2id using a maintained Go implementation and a PHC-style encoded string containing algorithm parameters and salt.
- Calibrate memory/time/parallelism in the target deployment at implementation time and set safe minimum/maximum password lengths. Do not silently truncate.
- Use cryptographically random per-password salts. A server-side pepper is optional only if managed separately in a secret manager; it must not become a required local-development secret without documentation.
- Compare/verify in bounded time and return the same public login error for unknown email and wrong password.
- Increment `users.auth_version` and revoke refresh families on password change, account disable/delete, or confirmed compromise.

The current encoded parameters are Argon2id v19, 64 MiB memory, three iterations, parallelism two, a 16-byte random salt, and a 32-byte result. Recalibration must preserve verification of existing PHC strings and remain within endpoint concurrency/resource budgets.

### 4.2 JWTs and refresh rotation

- Access and refresh tokens are both JWTs but use separate signing keys, audiences/purpose claims, code paths, and TTLs. Initial targets are 15 minutes access and 30 days refresh.
- For the single-verifier modular monolith, allowlisted HS256 with independently generated 256-bit-or-stronger keys is sufficient and avoids unnecessary asymmetric-key infrastructure. Never trust the token's `alg`; configure an exact algorithm allowlist.
- Claims include issuer, audience/purpose, `sub` user UUID, `jti`, session/family identifier, issued-at, not-before as needed, expiry, and `auth_version`. Do not include email or training/profile data.
- Signing keys come from deployment secrets. Rotation supports a bounded current/previous key window identified by an allowlisted `kid`; retirement waits no longer than the maximum token lifetime.
- Refresh JWT is set only in a `Secure`, `HttpOnly`, `SameSite=Lax` cookie scoped as narrowly as routing permits. It is never returned in JSON or stored by JavaScript.
- Access JWT is returned in login/refresh JSON, kept in browser memory, and sent in the Authorization header. Never persist it in local/session storage.
- Store only SHA-256 refresh-token hash, JTI, family, expiry, replacement, and revocation metadata. Rotate on every successful refresh using a row lock. The implemented conservative replay response to a replaced token revokes all active refresh tokens, increments `auth_version`, and emits a metadata-only security audit event.
- Logout revokes the current session/family and clears the cookie; a future logout-all flow will revoke all families and increment auth version.

The current single-instance slice accepts one active environment key per token purpose and therefore requires a coordinated restart/expiry window during key rotation. The planned `kid` current/previous window remains a release-hardening item; it was not added as speculative key-management infrastructure in the first auth slice.

### 4.3 Cookie-authenticated endpoint protection

Refresh and cookie-assisted logout routes check an exact trusted `Origin`/`Referer` policy in addition to `SameSite`, accept only POST, and have tight CORS credentials rules. Production serves frontend/API on one site. Access-token-authenticated writes do not rely on ambient cookies, substantially reducing CSRF exposure.

### 4.4 Abuse controls

- Rate-limit register/login/refresh/password routes by normalized account key and source IP with safe proxy handling.
- Use progressive backoff/temporary throttling rather than permanent attacker-controlled account lockout.
- Do not reveal account existence in login/reset behavior.
- Record stable failure/reuse codes and request IDs, never attempted passwords/tokens.

Current auth throttling is a per-process fixed-window limiter keyed by direct socket IP plus normalized email/digest. It deliberately ignores forwarded-IP headers until trusted proxies are configurable. A multi-replica/public deployment must add a shared edge or datastore limiter; the in-process limiter is defense in depth, not the only production abuse control.

## 5. Authorization and tenant isolation

- HTTP path/resource IDs select a candidate only. Application/persistence APIs also require the authenticated actor ID.
- Root tables carry `user_id`; children use tenant-safe composite FKs as described in `database-schema.md`.
- Every list/aggregate/tool query has a mandatory user predicate. Reviews reject functions whose only lookup input for private data is `id`.
- Cross-user, missing, and inaccessible-custom-exercise lookups return the same `404`/safe tool denial.
- System exercises are globally readable and immutable to normal users. Custom exercises are readable/writable only by their owner.
- The `program` module is the only writer of program tables, and `workout` the only writer of workout tables. Coach confirmation calls a public command; it never writes foreign tables directly.
- Account/session administration operates only on owned sessions. Privileged/admin roles are out of first-release scope, preventing accidental “temporary admin” bypasses.

Initial tenant enforcement is application predicates plus database composite FKs and exhaustive tests, not RLS. RLS is reconsidered before enterprise/shared-account/high-assurance use; if adopted, each pgx transaction must use safe `SET LOCAL` context and fail closed when absent.

## 6. API and input security

- Enforce HTTPS and HSTS in production; trust forwarded scheme/IP headers only from configured reverse proxies.
- Exact-origin CORS allowlist; never combine wildcard origin with credentials.
- Apply route-specific body limits, header limits, read/write/idle deadlines, and cancellation propagation.
- Decode one JSON object, reject unknown fields/trailing values, validate string lengths, enums, numeric ranges, UUIDs, time ranges, pagination limits, and IANA zones.
- Use parameterized pgx SQL only. Dynamic sort/filter fragments come from hard-coded allowlists, never direct request strings.
- Errors expose stable codes and request IDs, not SQL, stack traces, provider bodies, JWT reasons, or resource ownership.
- Apply `Cache-Control: no-store` to authenticated content. Use `nosniff`, frame-ancestor/frame protection, referrer policy, and a restrictive Content Security Policy suited to Next.js/Recharts.
- Escape/render coach and note text as plain text by default. Do not support raw HTML/Markdown HTML without a reviewed sanitizer and CSP tests.
- Idempotency and ETag checks prevent retry duplication and lost updates; keys are random/bounded and never used as authentication.

## 7. Database security

- Use a dedicated least-privilege runtime role. Production migrations use a separate controlled role/process; the application does not auto-migrate on startup.
- PostgreSQL is not exposed publicly. Require TLS where traffic crosses a host/network trust boundary.
- Set pgx pool/statement/lock/transaction timeouts and UTC session time zone.
- Foreign keys, checks, unique/partial indexes, root versions, row locks, and tenant-safe composite FKs enforce documented invariants.
- Encrypt production disks and backups, restrict backup access, define retention, and regularly test restore. Backup success without restore verification is insufficient.
- Do not copy production data into development/test. Use synthetic fixtures with no real emails, messages, or measurements.
- Account purge is an explicit, auditable job. Derived personal records and report/coach data follow documented cascading/retention behavior.

## 8. AI-specific security

### 8.1 Trust boundaries

User text, stored free-text notes, exercise descriptions, tool arguments proposed by the model, tool output reintroduced to the model, and final model content are all untrusted. A developer/system prompt is guidance, not access control.

The only security authority is backend code that:

- chooses an allowlisted tool name;
- injects actor context server-side;
- validates strict schema and domain values;
- authorizes every referenced resource;
- bounds time range, row count, fields, calls, tokens, and deadlines;
- minimizes/redacts output;
- enforces recommendation state/version/confirmation transactions.

Function schemas use `strict: true`, all properties required with nullable union where optional, and `additionalProperties: false`. Strict shape does not imply safe meaning, so backend validation remains mandatory. Parallel tool calls are initially disabled for deterministic authorization/audit.

Direct model context is limited to text the user explicitly submits to the coach and bounded text history from that owned conversation. Profile, program, workout, measurement, wellness, progress, and report facts come through safe backend tools. Weekly narrative gets only a specialized safe backend-context projection of the already stored deterministic report metrics.

Asynchronous processing never stores the original JWT. Admission writes owner only from verified context; every attempt reconstructs a limited principal from tenant-safe rows and rechecks active account, enabled/current AI notice, active conversation, and processing fencing token before provider/tools. Attempt UUID plus lease expiry fences late workers. Archive, AI disable, or account disable/delete cancels/fences work and forbids further calls/writes.

### 8.2 Prompt injection resistance

- Developer policy is static and versioned; never interpolate user/tool text into it.
- User content remains in a user message. Free-text fields in tool output are labeled as untrusted data and preferably omitted in favor of structured facts.
- Never obey text inside notes/tool output as tool instructions.
- There is no general web/MCP/file/shell/code tool, preventing exfiltration to a model-selected destination.
- Limit prompt length, output tokens, tool rounds, result size, date range, and retry count.
- Red-team attempts to select another user, request secrets/raw SQL, override policy, encode data into arguments, create huge ranges, and trick the coach into applying a proposal.

### 8.3 Program change approval

- `propose_program_change` is a pure run-local validator/canonicalizer with no durable or program write. Only after the final assistant response succeeds is its candidate persisted with that response as a typed `proposed` recommendation.
- The public confirmation endpoint is not a model tool and is called only by an explicit UI action.
- UI shows exact canonical before/after diff, rationale/limitations, target/base version, expiry, and that AI can be wrong.
- Confirmation requires access JWT, ownership, recommendation ETag, idempotency key, proposal digest, unexpired proposed state, visible exercises, valid typed operations, and exact current program version.
- Row locks plus one transaction apply the program command and set recommendation `applied`; any failure rolls back all changes. Concurrent/repeated confirmations cannot apply twice.
- A stale proposal performs no program write, commits only `proposed -> superseded`, and returns `409 recommendation_stale`; never auto-rebase or partially apply. Expired confirm similarly commits only `expired` and returns `recommendation_expired`.

### 8.4 Safety and fitness scope

- Moderate user input and output as an additional signal, not a substitute for authorization.
- Restrict coach behavior to general fitness/training planning. It must not diagnose injury/disease, prescribe treatment/medication, guarantee outcomes, or override acute warning signs.
- Injury, severe symptom, eating-disorder, self-harm, or emergency signals follow reviewed safe-response/escalation templates and do not create actionable program recommendations.
- Provide a visible report-feedback path and retain enough safe metadata to investigate without logging raw sensitive content.
- Send a stable privacy-preserving OpenAI `safety_identifier`, e.g. HMAC with a dedicated server secret over internal user UUID; never send email as the identifier.

See `ai-coach.md` for the complete orchestration and test cases.

## 9. Secrets and configuration

- Commit only `.env.example` placeholders when implementation begins; `.env`, `.env.*.local`, key files, dumps, and generated credentials are ignored.
- Secrets come from environment variables locally and an approved secret manager in production. Next.js public variables must be reviewed because `NEXT_PUBLIC_*` values are intentionally exposed.
- `OPENAI_API_KEY`, JWT keys, DB credentials, safety-ID HMAC key, audit-digest HMAC key, and any encryption keys are backend/deployment secrets. None is prefixed `NEXT_PUBLIC_`.
- Validate required configuration at startup without printing values. Error messages name a missing variable only.
- Use separate least-privilege credentials per environment. Rotate on a schedule and immediately after suspected exposure; inspect usage and invalidate compromised keys.
- Add secret scanning/pre-commit or CI detection, but never assume scanning makes committing a secret safe.

OpenAI officially recommends keeping API keys out of client-side environments and repositories and routing requests through a backend; the implementation follows that boundary. See [OpenAI production best practices](https://developers.openai.com/api/docs/guides/production-best-practices) and [OpenAI safety best practices](https://developers.openai.com/api/docs/guides/safety-best-practices).

## 10. Logging, telemetry, and audit

Runtime logs are structured JSON and use a central redaction policy. Never log:

- passwords/hashes/Authorization/Cookie headers;
- access or refresh JWTs, token hashes, signing/DB/OpenAI keys;
- raw request/response bodies for authenticated routes;
- raw coach prompts/messages, hidden reasoning, full tool arguments/results;
- unrestricted notes, body measurements, emails, IPs, or user agents.

Log safe request/route/status/duration/error data. AI operational fields may include pseudonymous user ID, conversation/message IDs, tool name, audit-key HMAC argument/result digest, row count/range, provider request ID, model, prompt/schema version, token counts, latency, and outcome. Do not treat raw SHA-256 as concealment for low-entropy measurements. Apply access controls and retention to logs.

Immutable business audit covers refresh reuse/revocation, auth-version changes, account deletion, workout reopen, program activation, coach tool denial, recommendation creation/confirm/reject/stale outcome, and report regeneration. Metadata is minimized and never duplicates raw sensitive content.

## 11. Privacy and OpenAI data controls

- Coach use is disabled until the user explicitly enables it after seeing the current versioned data-use notice. Disabling fences/cancels queued/in-flight work before further provider/tools and records the disable instant.
- GymTracker explicitly sends `store: false` in every initial, continuation, retry, and report-insight Responses request; adapter omission is a tested error. Keep conversation history in GymTracker PostgreSQL and send only a bounded relevant context.
- Responses continuations transiently carry all API-required prior output items, including opaque/encrypted reasoning items and function calls, plus matching call output. These items are never logged, exposed, or retained long-term; only user-visible content and safe metadata are persisted.
- Do not use OpenAI Conversations persistence by default. Do not send refresh/auth metadata, email, exact IP, secrets, or unrelated notes.
- `store: false` disables saved Response objects but does not by itself promise zero provider retention; organization/project abuse-monitoring and data-control terms still apply and must be accurately reflected in the privacy notice.
- API data is handled according to the deployment's OpenAI organization/project settings. Zero Data Retention, if contractually available and required, is a deployment decision verified before launch.
- Give users mechanisms to review/delete account data according to a published retention/purge policy. Do not claim immediate deletion until the backup/provider lifecycle is defined.

Official details: [OpenAI data controls](https://developers.openai.com/api/docs/guides/your-data) and [response retention/conversation state](https://developers.openai.com/api/docs/guides/conversation-state#data-retention-for-model-responses).

## 12. Frontend security

- No OpenAI SDK/key and no direct database connectivity in the frontend.
- Access JWT exists only in memory. Refresh bootstrap is same-site and cookie-protected.
- Treat all API strings as untrusted. React escaping remains enabled; do not use `dangerouslySetInnerHTML` for coach/notes.
- Zod is UX validation only; backend repeats every validation and authorization check.
- Confirm/reject are distinct controls with accessible labels and clear pending/success/error state. Disable duplicate clicks but rely on backend idempotency for correctness.
- Do not place secrets or sensitive payloads in URLs, analytics events, browser error reports, or query caches persisted to storage.
- Configure CSP and third-party scripts conservatively; no third-party analytics receives coach, body, wellness, or workout content by default.

## 13. Supply chain, CI, and infrastructure

- Pin/review direct dependencies and verify Go 1.22.2/Node 22 compatibility before adding them. Commit `go.sum` and npm lockfile once generated with actual dependencies.
- CI runs tests, Go vet/lint, TypeScript lint/type-check/build, OpenAPI lint/break detection, migration tests, dependency/vulnerability audit, secret scan, and container build/scan as implemented.
- GitHub Actions permissions default to read-only; grant narrow per-job permissions. Pin third-party actions to reviewed commit SHAs for production workflows.
- Build minimal non-root containers, use fixed base-image versions/digests with update process, read-only filesystem where feasible, dropped Linux capabilities, health checks, resource limits, and no embedded secrets.
- Compose is for local/testing; production secrets are not passed in committed Compose files. Database port is internal by default.
- Docker daemon access is an environment-owner concern. Never use `sudo` or alter host permissions from repository tasks.

## 14. Security verification gates

Required automated tests include:

- password/JWT purpose/algorithm/expiry/issuer/audience/auth-version validation;
- refresh rotation, replay, family revocation, concurrent refresh, and logout (plus logout-all when that route is added);
- cross-user IDOR matrices for every private REST endpoint and every backend tool;
- global versus custom exercise visibility/write rules;
- SQL/filter/cursor/body/unknown-field/range fuzz and negative tests;
- ETag lost-update and idempotency replay/body-mismatch/concurrency tests;
- workout lifecycle, immutable completion, reopen PR recalculation/report staleness;
- strict tool schema and separate domain validation failures;
- prompt-injection attempts and forbidden tool names/owner arguments/range escalation;
- proof that the OpenAI loop has no program write/apply/confirm path;
- recommendation confirm without explicit action/auth/ownership/current hash/version, concurrent confirms, rollback and stale proposal;
- log snapshots/assertions proving sensitive headers, tokens, key patterns, prompts, and values are redacted;
- DST weekly/daily boundary and report tenant-isolation fixtures.

Manual release review covers CSP/CORS/cookie attributes, TLS/proxy configuration, secret placement/rotation, backup restore, OpenAI project data controls, privacy copy/consent, rate limits/budgets, accessibility of confirmation diff, and incident contacts.

## 15. Incident response outline

1. Detect through provider usage alerts, refresh-reuse/auth anomalies, rate/budget alerts, audit events, or user reports.
2. Contain by disabling affected accounts, incrementing auth versions, revoking refresh families, rotating JWT/DB/OpenAI keys, and restricting routes/deployment as scoped.
3. Preserve minimized audit/log evidence with access controls; do not create new raw sensitive dumps.
4. Determine exposed data/period/actors and provider/dependency involvement.
5. Restore safely, verify credentials/config/tests, notify affected parties/regulators/providers where policy/law requires, and document decisions.
6. Add regression tests and update threat model/runbooks after remediation.

## 16. Residual risks and accepted trade-offs

- No initial RLS means a missing user predicate remains dangerous; composite FKs, typed persistence APIs, review, and exhaustive IDOR/tool tests reduce but do not eliminate this risk.
- App-managed context with `store: false` improves control/minimization but costs more tokens and requires careful summarization.
- A wide body-measurement table is type-safe but schema changes are needed for new metrics.
- In-memory access tokens reduce persistent theft but make refresh bootstrap and fully private SSR less convenient.
- HS256 is operationally simple for one verifier, but signing-key compromise grants minting ability; separate strong keys, strict algorithm checks, limited TTLs, rotation, and secret-manager controls are required.
- Read-only coach tools do not prompt for approval on every read. Starting a coach interaction is purpose-bound consent; allowlists, minimization, audit, and no core write tools limit impact.
- `parallel_tool_calls: false` and bounded context improve auditability/security at potential latency and answer-quality cost; change only after evaluation and threat review.
