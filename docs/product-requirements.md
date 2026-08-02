# GymTracker AI product requirements

Status: proposed design baseline

Last updated: 2026-08-02

## 1. Product summary

GymTracker AI is a personal strength-training web application. It combines program planning, fast workout logging, body and wellness tracking, progress analysis, weekly reporting, and an AI coach that can reason over a deliberately limited view of the authenticated user's data.

The product is advisory, not medical. AI output must be presented as a suggestion, explain its evidence and limitations, and never silently change user-owned training data.

## 2. Goals

- Let a user create and maintain a multi-day training program.
- Make recording a live workout, including weight, repetitions, and RIR, quick and reliable.
- Preserve a trustworthy history even after the underlying program changes.
- Show understandable trends for exercise performance, training volume, body mass, measurements, wellness, and personal records.
- Generate one reproducible report per user-defined week.
- Let an AI coach answer questions using relevant user data without granting the model direct database access.
- Require explicit, informed user confirmation before applying any AI-proposed program change.

## 3. Non-goals for the first release

- Social feeds, public profiles, leaderboards, coaching marketplaces, or shared team accounts.
- Native mobile applications or offline-first synchronization.
- Wearable integrations, meal-level nutrition planning, computer vision, or automatic exercise recognition. Daily aggregate calories/macronutrients and steps are in scope; foods, meals, recipes, and nutrient databases are not.
- Medical diagnosis, injury treatment, rehabilitation prescriptions, or emergency guidance.
- Fully autonomous training changes or AI writes to core user data.
- Microservices, event brokers, data warehouses, or plugin systems.
- Arbitrary user-authored formulas or an unrestricted AI query language.

## 4. Primary user

An authenticated adult strength-training user who follows a repeatable program, records workouts and optional body/wellness data, and wants evidence-based summaries and suggestions. The initial product supports one personal workspace per account and one preferred IANA time zone and unit system.

## 5. Product principles

- **User control:** the user can inspect, accept, or reject AI recommendations.
- **Historical truth:** completed workout facts are snapshots and are not rewritten by later program edits.
- **Minimum necessary data:** optional measurements and wellness data are collected only when useful to a user-visible feature.
- **Deterministic before generative:** metrics and charts are calculated in application code/SQL; AI explains them but does not invent them.
- **Fast common path:** starting a programmed workout and completing a set require minimal input.
- **Clear uncertainty:** missing data, insufficient history, model limitations, and failed report generation are visible.

## 6. Functional requirements

### 6.1 Account and profile

- **AUTH-01:** A user can register with a unique normalized email and password, log in, refresh a session, log out of the current session, and revoke all sessions.
- **AUTH-02:** Access and refresh credentials use separate JWT purposes and lifetimes; refresh tokens rotate and reuse is detected.
- **USER-01:** A user can view and update display name, IANA time zone, locale, preferred unit system, experience level, height, and an optional training-goal summary.
- **USER-02:** Every private operation is scoped to the authenticated user. A resource belonging to another user is not disclosed.
- **USER-03:** AI coach processing is disabled until the user enables it after seeing the current data-use notice. Disabling it cancels/fences pending work and prevents new provider calls and tools; the backend records the notice version and enable/disable instants.

### 6.2 Exercise catalogue

- **EXERCISE-01:** A user can search and filter a read-only global exercise catalogue.
- **EXERCISE-02:** A user can create, edit, archive, and reuse custom exercises visible only to that user.
- **EXERCISE-03:** Referenced exercises are not hard-deleted in a way that damages program or workout history.

### 6.3 Programs

- **PROGRAM-01:** A user can create, rename, describe, archive, and inspect programs.
- **PROGRAM-02:** A program contains ordered days; each day contains ordered exercise prescriptions.
- **PROGRAM-03:** A prescription supports target set count, repetition range, target RIR, rest duration, and notes.
- **PROGRAM-04:** A user can reorder days and exercises without recreating them.
- **PROGRAM-05:** At most one program is active per user in the first release. Activating another program atomically deactivates the previous one.
- **PROGRAM-06:** Program edits use optimistic version checks so concurrent UI or AI actions cannot overwrite newer changes.
- **PROGRAM-07:** Direct edits made by the user are allowed. AI-originated edits follow the confirmation workflow in section 6.10.

### 6.4 Workouts

- **WORKOUT-01:** A user can start a blank workout or instantiate one from an accessible program day.
- **WORKOUT-02:** Starting from a program copies the relevant prescription and exercise name into the workout snapshot.
- **WORKOUT-03:** A workout contains ordered exercises and ordered sets. A completed set records set type, at least one metric supported by the exercise (weight, repetitions, duration, or distance), optional RIR, completion time, and optional notes.
- **WORKOUT-04:** Weight is stored canonically in kilograms and RIR is constrained to `0.0` through `10.0`.
- **WORKOUT-05:** A workout moves through explicit states: `planned`, `in_progress`, and then `completed` or `cancelled`. Invalid state transitions are rejected.
- **WORKOUT-06:** Completing a workout validates completed-set data, sets a completion instant, rebuilds personal-record discoveries, and marks affected current reports stale in one transaction through narrow module ports.
- **WORKOUT-07:** A user can browse and filter their workout history by time range, status, program, and exercise.
- **WORKOUT-08:** Completed history remains readable if a source program or custom exercise is later archived.
- **WORKOUT-09:** A user can have at most one `in_progress` workout; a second start returns a visible conflict rather than silently abandoning or merging sessions.
- **WORKOUT-10:** A user can explicitly correct or delete an owned completed workout. Correction routes preserve completed status and require completion time to remain non-null and valid even when that instant is corrected, use optimistic concurrency, and trigger derived-data recalculation/staleness once those projections are implemented.

### 6.5 Body measurements and wellness

- **MEASUREMENT-01:** A user can record body mass and any subset of supported body-fat and circumference measurements at an explicit instant.
- **MEASUREMENT-02:** Measurements are stored in kilograms and centimetres and converted for display when the user prefers imperial units.
- **MEASUREMENT-03:** A user can create at most one daily wellness entry for the local civil day containing a supplied UTC observation instant. The implemented v1 entry can contain sleep duration, sleep quality, energy, steps, aggregate calories/protein/fat/carbohydrates, and notes. Existing schema fields for soreness, stress, mood, and resting heart rate remain reserved for a later transport extension.
- **MEASUREMENT-04:** The backend, not the client, calculates the exact start of that civil day from the current profile IANA zone and stores it as a UTC instant together with the zone used. The user updates their profile zone before logging travel-day data.

### 6.6 Progress and records

- **PROGRESS-01:** A user can view time-series data for working-set volume, set/repetition counts, maximum load, and estimated one-repetition maximum per exercise.
- **PROGRESS-02:** A user can view body-mass and circumference trends over a selected time range.
- **PROGRESS-03:** The system identifies auditable personal records from completed sets using a documented calculation version.
- **PROGRESS-04:** Chart endpoints return bounded, already aggregated series; the browser never downloads unbounded workout history to calculate charts.
- **PROGRESS-05:** Empty and insufficient-data states are explicit and must not be replaced by fabricated values.

### 6.7 Weekly reports

- **REPORT-01:** A report covers a half-open weekly interval `[period_start_at, period_end_at)` calculated from the user's IANA time zone and represented by UTC instants.
- **REPORT-02:** The deterministic report includes completed workouts, working sets, repetitions, volume, exercise performance, personal records, body trend, and wellness adherence when data exists.
- **REPORT-03:** The implemented manual generation is state-idempotent: an existing current ready report is returned unchanged, while a stale current artifact is regenerated as a new immutable revision. Deterministic generation is synchronous and serialized per user; durable asynchronous retry is deferred until AI narrative work has a concrete need.
- **REPORT-04:** AI-written insight is optional and is based only on the stored deterministic metrics. If OpenAI is unavailable, deterministic report data remains usable and the AI insight failure is visible.
- **REPORT-05:** A report stores an immutable input cutoff and metric snapshot so later corrections do not silently rewrite what was originally reported. Explicit regeneration creates a new report revision and retains the prior artifact.
- **REPORT-06:** Any source mutation affecting a generated period—including a newly completed/backdated workout, direct correction/deletion, or body/wellness create/update/delete—marks the current ready report stale. Metrics are read from one consistent database snapshot before optional AI insight.

### 6.8 AI coach conversations

- **COACH-01:** A user can create, rename, archive, and review their coach conversations and messages.
- **COACH-02:** The Go backend sends prompts to OpenAI; the OpenAI API key is never present in browser code or responses.
- **COACH-03:** The model obtains persisted profile, training, measurement, wellness, progress, and report facts only by requesting allowlisted, bounded backend tools. There is no generic SQL, HTTP, filesystem, or secret tool.
- **COACH-03A:** The only direct model context exception is the current user-submitted coach text and a bounded history from that same conversation. Profile, program, workout, measurement, wellness, progress, and report facts come through safe backend tools. Weekly insight receives only a safe backend-tool projection of stored deterministic report metrics.
- **COACH-04:** Tool execution derives ownership from verified authentication context and never accepts `user_id` as a model-controlled argument.
- **COACH-05:** The backend validates every tool argument and result, limits ranges/row counts/tool calls, and records safe audit metadata.
- **COACH-06:** Coach answers identify the observations used, distinguish facts from suggestions, and state when available data is insufficient.
- **COACH-07:** Coach content is moderated and constrained to fitness planning; medical or emergency questions receive an appropriate limitation/escalation message.
- **COACH-07A:** Archived conversations are read-only. Archiving, disabling AI, or deleting the account fences/cancels pending processing and prevents late workers from saving output.

### 6.9 Recommendation lifecycle

- **COACH-08:** A proposed program change is a typed recommendation containing a human-readable summary, rationale, structured diff, target program ID, and target program base version.
- **COACH-09:** The model-facing proposal path can result only in a `proposed` recommendation persisted with the completed assistant response. The tool loop itself cannot update program tables or make a proposal actionable before the response succeeds.
- **COACH-10:** The UI displays the exact proposed changes and offers separate confirm and reject actions. Viewing or continuing the chat does not count as confirmation.
- **COACH-11:** On confirmation, the backend rechecks authenticated ownership, recommendation state and expiry, payload/domain validity, exercise visibility, and the program base version.
- **COACH-12:** A valid confirmation applies the diff through the `program` module and marks the recommendation `applied` in one database transaction. Repeated confirmation is idempotent.
- **COACH-13:** A stale/conflicting proposal is not silently rebased; it remains unapplied and the user is asked to request a fresh proposal.
- **COACH-14:** Rejection records the decision without changing the program.

## 7. Core user journeys

### 7.1 Build and run a program

1. Register and set time zone/unit preferences.
2. Select global exercises or add custom exercises.
3. Create a program, days, and prescriptions; activate the program.
4. Start a workout from a program day; the backend creates snapshots.
5. Complete sets and finish the workout.
6. Review history, charts, and records.

### 7.2 Review a week

1. The backend calculates the user's weekly UTC boundaries from their saved IANA time zone.
2. It aggregates only facts whose instants fall in the half-open interval.
3. It stores deterministic metrics and optionally requests an AI explanation.
4. The user can inspect metrics, source links, missing-data notices, and coach insight status.

### 7.3 Apply an AI program proposal

1. The user asks the coach about their current program.
2. The model requests bounded read tools; the backend authorizes each call.
3. The model requests `propose_program_change`; the backend validates a run-local canonical candidate without a durable or program write.
4. After the final assistant response succeeds, the backend stores the candidate with that response as a `proposed` recommendation, and the UI renders its diff.
5. The user explicitly confirms or rejects it.
6. Only the confirmation endpoint can call the program mutation service.

## 8. Non-functional requirements

- **NFR-01 Security:** Follow `docs/security.md`; authorization and AI approval invariants are covered by integration tests.
- **NFR-02 Privacy:** Collect and send the minimum necessary data, support account-data deletion, redact sensitive logs, and default OpenAI requests to non-persistent response storage where supported.
- **NFR-03 Reliability:** Core logging and deterministic reports work when OpenAI is unavailable. Mutations are transactional and idempotent where retries are likely.
- **NFR-04 Performance:** Normal list queries are paginated. Chart and AI tool queries have bounded date windows and result sizes. Target service objectives will be set after representative load tests rather than guessed in design.
- **NFR-05 Accessibility:** Core flows are keyboard-operable, have visible focus, semantic labels, sufficient contrast, and do not rely on chart colour alone.
- **NFR-06 Observability:** JSON logs include timestamp, level, service, environment, request ID, route, status, duration, and safe error codes; metrics cover latency, errors, DB pool, report jobs, and OpenAI usage/failures.
- **NFR-07 Compatibility:** Backend dependencies must support Go 1.22.2; frontend tooling must support Node.js 22 and npm.
- **NFR-08 Time:** PostgreSQL stores every instant as `timestamptz`; database sessions and logs use UTC; API timestamps are RFC 3339 with `Z`. IANA zones are retained only to calculate/display local boundaries.
- **NFR-09 API evolution:** Breaking API changes require a new version; OpenAPI is kept synchronized with validation and behavior.
- **NFR-10 Testability:** Domain rules have unit tests; persistence, authorization, token rotation, state transitions, report aggregation, and AI confirmation have PostgreSQL-backed integration tests.

## 9. Success criteria for the first release

- A new user can complete the build-program-to-completed-workout journey without administrative intervention.
- Recorded completed sets can be retrieved unchanged after program edits.
- Cross-user access tests fail safely across every private module.
- Weekly metric fixtures produce deterministic expected output around time-zone and daylight-saving boundaries.
- The coach can answer representative progress questions using only approved tools.
- Automated tests prove that model/tool output alone cannot mutate a program and that only an explicit, authenticated confirmation applies a current proposal.
- The product remains usable for programs, workouts, measurements, and deterministic reports when OpenAI calls fail.

## 10. Open product questions

These questions do not block the architecture baseline but must be resolved before their feature is implemented:

- Whether self-registration needs email verification and password reset in the first public release.
- Estimated 1RM is resolved for v1 as Epley `weight × (1 + repetitions / 30)`, eligible only for non-warmup sets with 1–15 repetitions and identified by calculation version.
- Weekly reports are manually generated in the implemented backend. Automatic scheduling can be added later without changing period or revision semantics.
- The concrete OpenAI model and reasoning setting, chosen by evaluation for quality, latency, and cost and configured server-side rather than hard-coded into the domain.
