# GymTracker AI REST API contract

Status: implemented contract through backend exercise/program stage; workout is the current implementation contract and later modules remain design

Last updated: 2026-08-02

## 1. Scope and versioning

The Go backend exposes JSON REST endpoints under `/api/v1`. All routes are HTTPS-only outside local development. Breaking representation or behavior changes require a new API version; additive optional fields/endpoints may be added to v1 with synchronized OpenAPI and client validation.

Every `/api/v1` route except registration, login, and refresh requires a valid access JWT in:

```http
Authorization: Bearer <access-jwt>
```

No OpenAI endpoint, API key, internal coach tool, or database query endpoint is public.

Two minimal operational endpoints live outside the product API and are public to the trusted deployment network: `GET /health/live` (process event loop alive) and `GET /health/ready` (required local dependencies ready). They return only `200`/`503` and a generic status/request ID—never configuration, credentials, SQL/provider details, or user data.

## 2. Representation conventions

- JSON field names are `snake_case`.
- IDs are UUID strings.
- Instants are RFC 3339 UTC strings ending in `Z`; no local timestamp or bare calendar date is accepted for an instant.
- Canonical weights are kilograms and lengths are centimetres. Numeric fields are JSON numbers.
- `from` is inclusive and `to` exclusive. The server rejects `to <= from` and route-specific excessive ranges.
- Request content type is `application/json`; unknown fields, trailing JSON values, oversized bodies, and invalid UTF-8 are rejected. Duplicate-key rejection is not yet guaranteed by Go's standard JSON decoder, so clients must not send duplicate keys.
- PATCH is a documented partial object: missing means unchanged; `null` clears only a nullable field. JSON Merge Patch and JSON Patch are not implicitly supported.
- Private resources owned by another user are returned as `404`, not `403`, to avoid ID enumeration.
- Archive hides a resource from default collections and prevents normal mutation/use; direct GET by its owner remains available for inspection and returns its current ETag.
- Sensitive authenticated responses include `Cache-Control: no-store`.

### 2.1 Success envelope

Single resource:

```json
{
  "data": {
    "id": "018f4ea8-89af-7d75-b7e3-43d94f83292e"
  },
  "meta": {
    "request_id": "7ee593c5-04b4-4b6c-82ad-54d81095c99a"
  }
}
```

Collection:

```json
{
  "data": [],
  "meta": {
    "request_id": "7ee593c5-04b4-4b6c-82ad-54d81095c99a",
    "next_cursor": "eyJpZCI6IjAxOGYifQ"
  }
}
```

`next_cursor` is omitted when there is no next page. Limits are request parameters and are not repeated in the response.

`204 No Content` has no envelope/body. Creation returns `Location` and normally `201`; accepted asynchronous work returns `202` with the pending resource.

### 2.2 Errors

Errors use `application/problem+json`, compatible with RFC 9457:

```json
{
  "type": "https://gymtracker.example/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "detail": "Request contains invalid fields",
  "instance": "/api/v1/workouts/018f.../complete",
  "code": "validation_failed",
  "request_id": "7ee593c5-04b4-4b6c-82ad-54d81095c99a"
}
```

Stable `code` values, not English details, drive frontend behavior.

| Status | Meaning |
|---:|---|
| `400` | malformed JSON/query/cursor or unknown field |
| `401` | missing/invalid/expired access credential or inactive user |
| `403` | authenticated actor lacks a global capability; not used to reveal foreign ownership |
| `404` | resource absent, archived when excluded, or belongs to another user |
| `409` | business conflict, invalid lifecycle transition, reused idempotency key, stale AI proposal |
| `412` | supplied `If-Match` is stale |
| `413` | request body exceeds the route limit (`body_too_large`) |
| `415` | unsupported or missing request media type (`unsupported_media_type`) |
| `422` | syntactically valid request violates field/domain rules |
| `428` | required `If-Match` is missing |
| `429` | route/user/provider budget exceeded; includes `Retry-After` where known |
| `500` | unexpected internal failure with safe request ID |
| `503` | temporarily unavailable dependency/coach processing; core APIs remain available |

### 2.3 Request correlation

The client may send a valid bounded `X-Request-ID`; otherwise the backend creates one. Every response returns `X-Request-ID`, and the envelope/problem repeats it. Client-provided IDs are untrusted metadata, not authorization identifiers.

## 3. Pagination, filtering, ordering, and caching

Implemented growing collections use `?limit=50&cursor=<opaque>` with a maximum of 100. Cursors encode a stable sort key plus UUID as strictly validated opaque base64url data; they are not signatures or authorization artifacts. Clients must reuse a cursor only with the route/filter/order that produced it. Every page query independently reapplies ownership and filters; malformed cursors return `400`.

History defaults to descending event instant then ID; ordered aggregate children return ascending `position` and are not cursor-paginated within documented aggregate limits. `total` is omitted unless a concrete UI requirement justifies the count query.

Collection filters are allowlisted per endpoint. Repeating/unknown filters return `400`. Conditional GET may be added later, but authenticated responses are not browser/proxy-cacheable by default.

## 4. Idempotency and optimistic concurrency

Durable `Idempotency-Key` replay storage is not implemented yet and is therefore not advertised by implemented auth, profile, exercise, or program operations. `POST /programs/{id}/duplicate` is not transport-idempotent: after an ambiguous network failure the client must refetch the list before deciding whether to retry. Program activation is transactional and state-safe, but a retry with the consumed ETag receives `412` and must refetch.

The workout slice does not advertise `Idempotency-Key`: durable replay middleware is not available yet. Creating a workout, exercise, or set after an ambiguous transport failure requires a refetch before retrying. Workout completion is instead state-idempotent under a row lock: the first valid request performs the transition, while any concurrent or later request against that already completed owned workout returns the unchanged current aggregate without repeating derived effects. The initial transition still requires the current root ETag.

When durable replay is introduced for later business slices, it is intended for operations such as:

- create a body measurement or wellness entry;
- send or retry a coach message;
- generate or regenerate a weekly report;
- confirm or reject an AI recommendation.

The future mechanism must scope keys by authenticated user, method, and canonical path, bind them to a canonical request hash, and handle completed replay and simultaneous processing. It must not be approximated with process memory.

Mutable aggregates return strong ETags such as:

```http
ETag: "program:018f4ea8-89af-7d75-b7e3-43d94f83292e:7"
```

Program-tree mutations require the current program ETag; workout-tree mutations require the current workout ETag. Profile/AI settings, custom exercise, body measurement, wellness, conversation, recommendation, and report mutations require their resource ETag where marked below. A nested change increments the root version once for the request.

## 5. Auth module

Refresh JWTs are accepted only from the configured `HttpOnly; SameSite=Lax` cookie (`Secure` is mandatory in production). Access tokens are returned in JSON and auth responses use `Cache-Control: no-store`. Refresh and logout additionally require an exact allowed `Origin` or `Referer`.

| Method | Route | Auth | Input/result |
|---|---|---|---|
| `POST` | `/auth/register` | public | email/password; `201` user + access token and refresh cookie |
| `POST` | `/auth/login` | public | email/password; `200` user + access token and new refresh cookie |
| `POST` | `/auth/refresh` | refresh cookie | no JSON body; `200` access token + rotated refresh cookie |
| `POST` | `/auth/logout` | access + refresh | revoke current family and clear cookie; `204` |

Credentials have a 320-byte maximum normalized email and a password of 12–128 Unicode characters and at most 256 bytes. Token response fields are `user_id`, `email`, `access_token`, and `access_expires_at`; refresh token is never in JSON. Unknown email and wrong password both return the same `401 invalid_credentials` problem. Password-reset/email-change/session-list/logout-all routes are intentionally absent until their complete policies are implemented.

## 6. User module

| Method | Route | Concurrency | Behavior |
|---|---|---|---|
| `GET` | `/profile` | returns ETag | current authenticated user's profile |
| `PATCH` | `/profile` | `If-Match` required | strict partial profile update; `200` |
| `POST` | `/profile/import` | `If-Match` required | strict transactional JSON import and optional initial body measurement; `200` |

Profile fields are `name`, `sex`, `birth_date`, `height_cm`, `goal`, `experience_level`, `training_frequency`, `timezone`, `unit_system`, `sleep_hours_average`, imported `notes`, `version`, and UTC timestamps. Nullable PATCH fields can be cleared with JSON `null`; `timezone` and `unit_system` cannot be null. Supported goals are `muscle_gain`, `weight_loss`, `recomposition`, `strength`, and `maintenance`; levels are `beginner`, `intermediate`, and `advanced`.

Profile ETags have the form `"profile:{user_id}:{version}"`. Import accepts exactly the fields in `docs/openapi.yaml`; unknown top-level/nested fields, trailing JSON, and empty imports are rejected. Notes are trimmed, bounded, and replaced only when `notes` is present. Weight or circumference data creates one `body_measurements` row in the same transaction with `source=import`; `biceps_cm` initializes both upper arms. Owner and measurement time always come from the backend.

## 7. Exercise module

| Method | Route | Concurrency | Behavior |
|---|---|---|---|
| `GET` | `/exercises` | — | visible global + custom catalogue; cursor list |
| `POST` | `/exercises` | — | create custom exercise; `201` |
| `GET` | `/exercises/{id}` | returns ETag | visible exercise |
| `PATCH` | `/exercises/{id}` | `If-Match` required | edit owned custom exercise; system returns `403` |
| `DELETE` | `/exercises/{id}` | `If-Match` required | archive owned custom exercise; `204` |

List filters are `q`, `scope=all|system|mine`, `muscle_group`, `type`, `equipment`, `tracks_weight`, `tracks_repetitions`, `tracks_time`, `tracks_distance`, `include_archived`, `limit`, and `cursor`. Unknown or repeated query fields are rejected. Search is a case-normalized literal substring search, so `%` and `_` have no wildcard meaning. Direct GET of an archived owned custom exercise remains available for inspection. System exercises have `owner_user_id: null`, are read-only, and are never visible across ownership predicates as another user's custom exercise.

Types are `strength`, `cardio`, `stretching`, `bodyweight`, and `isometric`. Equipment is `barbell`, `dumbbell`, `machine`, `cable`, `pullup_bar`, `parallel_bars`, `bodyweight`, or `other`. At least one tracking capability must be enabled.

## 8. Program module

### 8.1 Routes

| Method | Route | Behavior |
|---|---|---|
| `GET`, `POST` | `/programs` | cursor list / create draft (`201`) |
| `GET`, `PATCH`, `DELETE` | `/programs/{id}` | full aggregate read / edit / archive; mutations require `If-Match` |
| `POST` | `/programs/{id}/duplicate` | create an independent draft with fresh IDs; optional `{ "name": ... }`; `201` |
| `POST` | `/programs/{id}/activate` | atomically replace the active program; empty body and `If-Match`; `200` |

List filters are `status`, `include_archived`, `limit`, and `cursor`; unknown/repeated fields are rejected. List rows are summaries without `days`; item GET and mutation responses contain the full aggregate.

Days and their exercises are supplied as bounded ordered arrays on create or in PATCH `days`. Positions must be exactly contiguous and one-based in array order. Sending `days` replaces the active tree transactionally: old day/item rows are archived, new rows receive fresh UUIDs, and the root version increments once. Omitting `days` leaves the tree unchanged; an empty array intentionally makes a draft non-activatable and is rejected for an already active program.

Activation locks all of the user's program roots in stable UUID order, validates the target aggregate and referenced exercises, deactivates the old active root, and activates the target in one transaction. The partial unique index on active programs is the final authority. Archived programs cannot be activated. Clients must invalidate/refetch the program collection because replacement also increments the previous active root's version.

DELETE archives rather than physically deletes. Full-tree replacement also archives superseded children. These rules, plus `NO ACTION` history foreign keys and workout snapshots, permit active-program edits without breaking instantiated or completed workouts. A confirmed future AI proposal may edit a program only through the coach confirmation flow.

### 8.2 Program representation

```json
{
  "id": "018f4ea8-89af-7d75-b7e3-43d94f83292e",
  "name": "Upper / Lower",
  "description": null,
  "goal": "Strength",
  "status": "active",
  "version": 7,
  "days": [
    {
      "id": "018f4ec2-4c69-7aa6-a820-686d8d132684",
      "position": 1,
      "name": "Upper A",
      "notes": null,
      "exercises": [
        {
          "id": "018f4ed1-8391-7e66-92c8-1014e0d9c908",
          "exercise_id": "018f4edb-a398-73d7-a474-382c893a8e16",
          "position": 1,
          "working_sets": 3,
          "target_reps_min": 5,
          "target_reps_max": 8,
          "target_rir": 2.0,
          "rest_seconds": 180,
          "notes": null
        }
      ]
    }
  ],
  "created_at": "2026-08-02T12:00:00Z",
  "updated_at": "2026-08-02T12:30:00Z"
}
```

List endpoints may return a summary without nested days. `GET /programs/{id}` returns the full bounded aggregate.

## 9. Workout module

### 9.1 Routes

| Method | Route | Behavior |
|---|---|---|
| `GET`, `POST` | `/workouts` | filtered cursor history summaries / create an `in_progress` or explicit `planned` workout; `201` on create |
| `GET` | `/workouts/active` | full current `in_progress` aggregate, or `200` with `data: null` when none exists |
| `GET` | `/workouts/export.csv` | bounded filtered CSV export without a JSON envelope |
| `GET`, `PATCH`, `DELETE` | `/workouts/{id}` | full aggregate read / strict partial edit / physical deletion of an owned workout in any status; PATCH/DELETE require root workout `If-Match` |
| `POST` | `/workouts/{id}/complete` | state-idempotent `in_progress -> completed`; root workout `If-Match`; `200` |
| `POST` | `/workouts/{id}/exercises` | add an exercise; root workout `If-Match`; `201` |
| `PATCH`, `DELETE` | `/workout-exercises/{id}` | edit/reorder or remove an item; containing workout `If-Match`; `200`/`204` |
| `GET` | `/workout-exercises/{id}/previous-result` | previous completed occurrence for the same exercise relative to this item; nullable `data`; `200` |
| `POST` | `/workout-exercises/{id}/sets` | append/insert a performed set; containing workout `If-Match`; `201` |
| `PATCH`, `DELETE` | `/workout-sets/{id}` | edit/reorder or remove a set; containing workout `If-Match`; `200`/`204` |

List filters are `status`, `from`, `to`, `program_id` (resolved through the source day), `exercise_id`, `limit`, and `cursor`. Unknown or repeated fields return `400 invalid_query`. `event_at = started_at ?? scheduled_at ?? created_at` is the filtering/default descending ordering/cursor instant, so planned rows have deterministic range semantics. List rows are summaries; item and active responses contain the full ordered aggregate.

Create defaults to `status: in_progress`; `started_at` defaults to the server clock. An explicit `status: planned` requires `started_at` to be absent or null and may include `scheduled_at`. The request chooses exactly one source mode:

```json
{
  "program_day_id": "018f4ec2-4c69-7aa6-a820-686d8d132684",
  "status": "in_progress",
  "started_at": "2026-08-02T12:00:00Z",
  "scheduled_at": null,
  "name": null,
  "difficulty": null,
  "energy": null,
  "mood": null,
  "comment": null,
  "has_pain": false,
  "discomfort": null
}
```

or a non-empty ad-hoc `name` with `program_day_id: null`. Program-day creation copies the program/day identity, program version, ordered exercise-name and tracking-capability snapshots, rest targets, prescriptions, and planned target sets. Later program or exercise edits cannot modify those snapshots.

PATCH permits `planned -> in_progress|cancelled` and `in_progress -> cancelled`; only the completion endpoint can enter `completed`. A completed workout remains completed but its timestamps, scores, comment, pain/discomfort fields, exercises, and sets may be corrected directly with the current ETag. In the current slice dynamic metrics immediately reflect those source-set corrections and the workout version increments once. Once record/report/audit modules are implemented, their narrow ports must join the same transaction to recalculate affected projections, mark old/new periods stale, and append minimized audit metadata. Cancelled workouts are immutable except for explicit deletion. DELETE is explicitly allowed for an owned workout in any status and cascades its children; future rebuildable projections and affected reports must be handled in the same transaction once their modules exist.

Starting an explicit or default `in_progress` workout, or moving a planned workout to `in_progress`, while another is active returns `409 workout_already_in_progress`; the backend never merges or cancels one implicitly.

Completion accepts an optional JSON object containing `completed_at`, `difficulty`, `energy`, `mood`, `comment`, `has_pain`, and `discomfort`; an absent `completed_at` uses the server clock and explicit `completed_at: null` is invalid. For the other nullable fields, omission preserves metadata already saved on the unfinished workout while explicit null clears it. Empty workouts may be completed. Every call still requires a syntactically valid root `If-Match` naming that workout. For the initial transition its version must be current. Under the workout row lock, an already completed owned workout returns `200` with the unchanged current aggregate and performs no version, record, report, or audit writes; this no-write branch deliberately runs before version comparison and therefore accepts any same-root workout version, including the ETag consumed by the original completion. Missing, malformed, or wrong-root validators still fail, and other stale mutations return `412` normally.

### 9.2 Workout and set fields

`difficulty`, `energy`, and `mood` are nullable integer scores from 1 through 10. `comment` and `discomfort` are nullable bounded text. `has_pain` records the user's answer; omission leaves it null, while explicit true or false records an answer.

Exercise items contain `position`, exercise ID/name and tracking-capability snapshots, read-only nullable `rest_seconds`, optional `comment`, and ordered sets. POST accepts `exercise_id`, optional insertion `position`, and optional `comment`; PATCH accepts only `position` and `comment`. Exercise identity and prescription-rest snapshots are not rewritten in place; correcting the selected exercise uses delete/add. Adding or moving an item shifts sibling positions transactionally. A newly selected exercise must be visible and non-archived; actual set metrics are checked against the snapshotted `tracks_weight`, `tracks_repetitions`, `tracks_time`, and `tracks_distance` capabilities.

Set responses include the server-derived `status`: copied program targets begin as `planned`, a set with actual metrics is `completed`, and untouched planned targets become `skipped` when the workout is completed. Clients do not write `status` directly. Set writes use `set_number`, canonical `weight_kg`, `reps`, `rir`, `warmup`, `failure`, `duration_seconds`, `distance_meters`, nullable `note`, and UTC `performed_at`. `set_number` and `performed_at` default respectively to append position and the server clock. At least one of weight, repetitions, duration, or distance is required for a performed set. Adding actual metrics to a planned set makes it completed; a completed set must retain at least one actual metric after PATCH, so removing its last metric requires deleting the set. At the persistence boundary `warmup: true` maps to internal `set_type = warmup`, `failure: true` maps to `set_type = failure`, and a newly created set with both flags false maps to `set_type = working`; both flags true are rejected. Existing internal non-warmup/non-failure subtypes project both flags as false and are preserved by patches that do not reclassify the set. Unsupported metrics return `422 metric_not_tracked`.

```json
{
  "id": "018f4f22-4a62-77a2-8a9f-c19db7b6f841",
  "workout_exercise_id": "018f4f10-3e47-75d9-a1d1-e09557960246",
  "set_number": 1,
  "status": "completed",
  "target_weight_kg": 100.0,
  "target_reps_min": 5,
  "target_reps_max": 8,
  "target_rir": 2.0,
  "weight_kg": 100.0,
  "reps": 7,
  "rir": 1.5,
  "warmup": false,
  "failure": false,
  "duration_seconds": null,
  "distance_meters": null,
  "note": null,
  "performed_at": "2026-08-02T13:14:52Z",
  "volume_kg": 700.0,
  "estimated_1rm_kg": 123.333
}
```

Volume is `weight_kg * reps` only for performed non-warmup sets. Estimated 1RM uses Epley, `weight_kg * (1 + reps / 30)`, only for a non-warmup set with positive weight and 1 through 15 repetitions. Sets above 15 repetitions remain valid but return `estimated_1rm_kg: null`.

### 9.3 Previous result and CSV

`GET /workout-exercises/{id}/previous-result` anchors both ownership and the historical cutoff in the current item. It finds the latest earlier completed workout containing the same exercise, ordered deterministically by workout event instant and ID, and returns its IDs/timestamps plus performed sets in ascending set-number order. No earlier result is a successful `200` with `data: null`; only a missing or foreign anchor returns `404`.

CSV accepts the list filters except `limit` and `cursor`, is ordered chronologically by workout event instant/ID, exercise position, and set number, and emits one row per set. Exercise/workout rows without sets remain present with empty set columns. The stable header is:

```csv
workout_id,workout_name,status,source_program_id,source_program_day_id,event_at,scheduled_at,started_at,completed_at,cancelled_at,difficulty,energy,mood,comment,has_pain,discomfort,workout_volume_kg,exercise_position,workout_exercise_id,exercise_id,exercise_name,set_number,set_status,weight_kg,reps,rir,warmup,failure,duration_seconds,distance_meters,note,performed_at,set_volume_kg,estimated_1rm_kg
```

The response is `text/csv; charset=utf-8` with `Content-Disposition: attachment`, `Cache-Control: no-store`, and `X-Request-ID`, but no JSON envelope. Values use canonical units, RFC 3339 UTC, dot decimals, lowercase booleans, and empty cells for null. Fields follow RFC 4180 quoting/CRLF rules. A user-controlled text cell whose first non-space character is `=`, `+`, `-`, or `@` is prefixed with an apostrophe before CSV quoting to prevent spreadsheet formula execution. Export is capped at 100,000 data rows; a larger match fails before CSV output with `422 export_too_large`, and the client must narrow its filters. Invalid queries and other pre-stream failures still return the standard problem JSON.

## 10. Measurement module

### 10.1 Body measurements

| Method | Route | Behavior |
|---|---|---|
| `GET`, `POST` | `/body-measurements` | range-filtered cursor list / create event (`201`, `Idempotency-Key` required) |
| `GET`, `PATCH`, `DELETE` | `/body-measurements/{measurement_id}` | GET returns item ETag; PATCH/DELETE require `If-Match` |

Filters: `from`, `to`, `metric`, and cursor. Requests use the canonical fields listed in `database-schema.md` and require at least one numeric measurement. The server, not the client, determines ownership/source policy. Create/update/delete whose old or new `measured_at` falls in a generated report period marks every affected current report stale atomically.

### 10.2 Daily wellness

| Method | Route | Behavior |
|---|---|---|
| `GET`, `POST` | `/daily-wellness` | bounded range list / create (`201`, `Idempotency-Key` required) |
| `GET`, `PATCH`, `DELETE` | `/daily-wellness/{entry_id}` | GET returns item ETag; PATCH/DELETE require `If-Match` |

Create supplies `observed_at` as a UTC instant plus wellness values. The backend uses the current profile IANA zone to calculate the first valid instant of that local civil day, stores it as `day_start_at`, and stores the zone as `timezone_at_entry`; neither boundary nor zone is client authority. Uniqueness conflict returns `409 wellness_already_exists`. A convenience query `GET /daily-wellness?from=&to=` uses stored UTC boundaries. Create/update/delete marks every current report covering the old or new stored day boundary stale atomically.

## 11. Progress module

Progress endpoints are read-only derived views. They never accept client writes to `personal_records`.

| Method | Route | Key query parameters |
|---|---|---|
| `GET` | `/progress/summary` | `from`, `to` |
| `GET` | `/progress/body` | `metric=weight_kg|body_fat_percent|neck_cm|chest_cm|waist_cm|hips_cm|left_upper_arm_cm|right_upper_arm_cm|left_thigh_cm|right_thigh_cm|left_calf_cm|right_calf_cm`, `from`, `to`, `granularity` |
| `GET` | `/progress/training-volume` | `metric=volume|working_sets|repetitions`, `from`, `to`, `granularity`, optional program/exercise |
| `GET` | `/progress/exercises/{exercise_id}` | `metric=e1rm|volume|max_weight|reps`, `from`, `to`, `granularity` |
| `GET` | `/progress/personal-records` | `exercise_id`, `record_type`, `from`, `to`, cursor |
| `GET` | `/progress/personal-records/{record_id}` | source set/workout link and calculation metadata |

Series response data has `metric`, `unit`, `from`, `to`, `granularity`, and bounded `points: [{ "at": ..., "value": ... }]`. Every deterministic calculation, especially estimated 1RM, exposes a `calculation_version`. No interpolation occurs across missing periods unless a representation explicitly marks generated empty buckets.

## 12. Report module

| Method | Route | Behavior |
|---|---|---|
| `GET` | `/reports/weekly` | current reports by default; cursor list; filters `from`, `to`, `status`, `include_revisions` |
| `POST` | `/reports/weekly` | request generation; `Idempotency-Key` required; `202` pending revision |
| `GET` | `/reports/weekly/{report_id}` | immutable revision; returns report ETag/status |
| `POST` | `/reports/weekly/{report_id}/regenerate` | create next revision from the current terminal revision only; `Idempotency-Key` + report `If-Match`; `202`, otherwise `409` |

Generate request:

```json
{
  "week_containing_at": "2026-07-29T12:00:00Z",
  "include_ai_insight": true
}
```

The backend derives `[period_start_at, period_end_at)` from the user's current IANA zone and stores that zone. Ordinary generate returns the existing `pending`, `generating`, or `ready` current report for the same period (`202` while pending/generating, `200` when ready). A current `failed` or `stale` report requires regenerate. Regenerate is allowed for current terminal `ready`, `failed`, or `stale` and always inserts a new `pending` revision; transient runner retries stay on that pending revision until attempts are exhausted.

First generation has no prior artifact, so its pending row is current. During regeneration, the prior artifact remains current while the replacement pending revision is returned directly and shown as replacement work. Only when replacement reaches `ready` does one transaction switch `is_current`; a failed replacement cannot hide the previous usable report.

The runner captures `input_data_through_at` and all deterministic aggregates in one short `REPEATABLE READ` database transaction, then commits the metric snapshot. Optional narrative insight runs afterward and receives only a specialized safe backend-context projection of the stored deterministic metrics, never live/raw training rows. Any later workout completion/direct correction/deletion or body/wellness create/update/delete affecting the interval marks the current ready report `stale`.

Report fields include UTC period bounds, zone, revision/current flag, generation status, deterministic `metrics` with schema version and input cutoff, attempt/retry metadata (`retryable`, optional `next_attempt_at`), AI insight/status, model/prompt version, error code, generated instant, and links to relevant progress/history filters. Metrics may be ready even when `ai_insight_status = "failed"`.

Initial `metrics_schema_version = 1` has a closed object containing:

- `totals`: `completed_workouts`, `working_sets`, `repetitions`, `volume_kg`;
- `exercise_summaries[]`: exercise ID/name snapshot, working sets, repetitions, volume, max weight, optional calculation-versioned estimated 1RM/change;
- `personal_records[]`: record ID/type/value/unit, exercise and achieved instant;
- `body`: sample counts and optional start/end/change for weight/body metrics (null plus coverage when insufficient);
- `wellness`: days logged/expected and optional averages for sleep, quality, energy, stress, soreness and mood;
- `coverage`: source cutoff and explicit missing/insufficient-data flags.

Unknown metrics fields are rejected by the versioned backend schema when writing. Narrative insight may quote/explain this object but cannot change it.

## 13. Coach module

### 13.1 Conversations and messages

| Method | Route | Behavior |
|---|---|---|
| `GET`, `POST` | `/coach/conversations` | cursor list (`status`, `include_archived`) / create (`201`) |
| `GET`, `PATCH`, `DELETE` | `/coach/conversations/{conversation_id}` | GET (including owned archived) returns ETag; PATCH/DELETE require `If-Match`; DELETE archives (`204`) |
| `GET`, `POST` | `/coach/conversations/{conversation_id}/messages` | cursor list / persist user + pending assistant (`Idempotency-Key` required) and return `202` |
| `GET` | `/coach/conversations/{conversation_id}/messages/{message_id}` | poll pending/processing/completed/failed status |
| `POST` | `/coach/conversations/{conversation_id}/messages/{message_id}/retry` | active conversation only; failed retryable assistant to pending; `Idempotency-Key`; `202` |

Message request:

```json
{
  "client_message_id": "018f5011-763a-795e-9064-60f8163130ca",
  "content": "Review my squat progress and suggest next week's targets."
}
```

The frontend polls pending assistant message status with bounded backoff. Resources expose `retryable` and optional `next_attempt_at`; provider `429`/transient failure after accepted `202` appears here as safe status/error metadata, while HTTP `429` is only admission/user-budget rejection before queueing. SSE can be added additively later, but v1 correctness does not require it. Message role, submitted user content, client ID, sequence, and completed assistant content are immutable; processing status/attempt/lease/error metadata changes through `pending -> processing -> completed|failed|cancelled`, explicit `failed -> pending` retry, and fenced `processing -> pending` lease recovery. Raw tool calls, hidden reasoning, system prompts, and unrestricted provider errors are not returned.

Archived conversations remain directly readable but are otherwise read-only; new message/retry returns `409 conversation_archived`. Archive invalidates the processing fence, cancels pending assistant work, attempts cancellation of in-flight provider requests, and prevents all later tool/provider calls and late writes.

### 13.2 Recommendations

| Method | Route | Behavior |
|---|---|---|
| `GET` | `/coach/recommendations` | cursor list; filters conversation/program/status |
| `GET` | `/coach/recommendations/{recommendation_id}` | exact proposal/diff, rationale, expiry, base version; returns ETag |
| `POST` | `/coach/recommendations/{recommendation_id}/confirm` | explicit user confirmation and synchronous atomic apply; `Idempotency-Key` + `If-Match`; `200` |
| `POST` | `/coach/recommendations/{recommendation_id}/reject` | reject without program mutation; `Idempotency-Key` + `If-Match`; `200` |

Recommendation representation:

```json
{
  "id": "018f5075-b0fb-7396-8ee5-4a7bda970159",
  "recommendation_type": "program_change",
  "status": "proposed",
  "summary": "Reduce squat volume for one week",
  "rationale": "Completed-set RIR declined while soreness increased.",
  "target_program_id": "018f4ea8-89af-7d75-b7e3-43d94f83292e",
  "expected_program_version": 7,
  "proposal_hash": "sha256-base64url-value",
  "payload_schema_version": 1,
  "changes": [
    {
      "operation": "update_prescription",
      "program_day_exercise_id": "018f4ed1-8391-7e66-92c8-1014e0d9c908",
      "before": { "target_sets": 4, "target_rir": 2.0 },
      "after": { "target_sets": 3, "target_rir": 3.0 }
    }
  ],
  "expires_at": "2026-08-09T12:00:00Z",
  "created_at": "2026-08-02T12:00:00Z"
}
```

Confirm request repeats the proposal digest shown by the UI:

```json
{
  "proposal_hash": "sha256-base64url-value"
}
```

Confirmation reauthenticates through the access JWT, loads by `(id, user_id)`, locks recommendation/program, checks `proposed`, expiry, digest, payload schema/domain rules, exercise visibility, and exact program version. It then calls the `program` application service and marks the recommendation `applied` in one transaction. The AI tool loop has no confirm/apply tool and cannot call this route. Response header ETag is the new recommendation ETag; body includes `applied_program_version` and program link (its ETag comes from a separate GET).

A base-version mismatch changes only `proposed -> superseded` under the recommendation lock, commits no program mutation, then returns `409 recommendation_stale` with the new recommendation ETag. If `expires_at <= now`, confirm similarly changes only the status to `expired` and returns `409 recommendation_expired`. Periodic cleanup also expires proposals.

Any confirm of an already `applied` recommendation by the same owner with the same proposal hash returns the original `200` applied result without writes, even with a new idempotency key; a mismatched hash returns `409 proposal_hash_mismatch`. Reject transitions only `proposed -> rejected`; repeated same-owner reject returns the rejected result without program writes.

## 14. Lifecycle summary

| Resource | Allowed transitions |
|---|---|
| User | `active -> disabled|deleted`; controlled restore policy only |
| Program | `draft|inactive -> active`; `active -> inactive`; non-archived -> `archived` |
| Workout | create as `planned|in_progress`; `planned -> in_progress|cancelled`; `in_progress -> completed` through complete or `cancelled` through PATCH; `completed` remains directly correctable; cancelled tree is immutable; any status may be deleted |
| Assistant message | `pending -> processing -> completed|failed`; `failed -> pending` explicit retry; fenced `processing -> pending` lease recovery; pending/processing -> `cancelled` on archive/AI disable/account disable-delete |
| Recommendation | `proposed -> applied|rejected|expired|superseded` |
| Weekly report revision | `pending -> generating -> ready|failed`; fenced `generating -> pending` transient retry/lease recovery; `ready -> stale` after any affecting source mutation |
| AI insight | `not_requested|pending -> ready|failed` |

Invalid transitions return `409`; invalid field values return `422`.

### 14.1 Stable conflict/error codes

| Situation | Status/code |
|---|---|
| Active replacement ID absent/wrong | `409 active_program_replacement_required` / `active_program_changed` |
| Workout create/start while another is active | `409 workout_already_in_progress` |
| Workout transition or cancelled-tree mutation is disallowed | `409 invalid_workout_state` |
| Set supplies a metric not tracked by its exercise snapshot | `422 metric_not_tracked` |
| CSV selection exceeds 100,000 data rows | `422 export_too_large` |
| Duplicate body measurement instant | `409 measurement_already_exists` |
| Duplicate calculated wellness civil day | `409 wellness_already_exists` |
| Archived conversation message/retry | `409 conversation_archived` |
| AI disabled/notice stale at admission | `409 ai_coach_disabled` / `ai_notice_outdated` |
| Proposal digest differs | `409 proposal_hash_mismatch` |
| Proposal expired/rejected/superseded/stale program | `409 recommendation_expired` / `recommendation_rejected` / `recommendation_superseded` / `recommendation_stale` |
| Regenerate a non-current or non-terminal report | `409 report_not_current` / `report_not_regeneratable` |
| Idempotency key processing/reused with other body | `409 idempotency_in_progress` / `idempotency_key_reused` |
| Aggregate ETag missing/stale | `428 precondition_required` / `412 precondition_failed` |

## 15. OpenAPI requirements for implementation

The future committed OpenAPI 3.1 document must be the complete machine-readable version of this design and include:

- security schemes for bearer access JWT and refresh cookie;
- all request/response/problem schemas with `additionalProperties: false` where appropriate;
- examples for canonical units, UTC timestamps, pagination, ETags, and idempotency;
- operation IDs grouped by the nine module tags;
- documented status/header combinations (`Location`, `ETag`, `If-Match`, `Idempotency-Key`, `Retry-After`, `X-Request-ID`);
- min/max/length/enum constraints synchronized with Zod, Go validation, and PostgreSQL checks;
- CI linting, behavior/contract tests, and a breaking-change check against the main-branch baseline.

If implementation discovers that this prose contract is ambiguous, reconcile this document and OpenAPI before adding a silent transport convention.
