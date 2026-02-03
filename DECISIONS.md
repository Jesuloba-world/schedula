## DECISIONS

This file records product and technical decisions made under ambiguous requirements, with brief rationale and trade-offs. It is intentionally updated as the project evolves.

Current scope: Appointment Scheduling only (one-off appointments + recurring series creation and occurrence listing, with a scheduling-friendly display).

## Appointment Scheduling (MVP)

### Decision 1: What data an appointment contains
Choice:
Appointment has these fields:
1. id (server-generated UUIDv7 by default; deterministic when Idempotency-Key is provided)
2. user_id (string identifier; no auth in MVP)
3. title (required)
4. notes (optional)
5. start_time (required)
6. end_time (required)
7. created_at (server-generated if unset)
8. updated_at (server-generated if unset)

Rationale:
Typical scheduling use cases require a human-readable label and a time range. A user scope keeps the model realistic without requiring full authentication. Notes support common “extra details” needs without expanding scope into invites, attendees, or locations.

Trade-offs considered:
1. Location, attendees, and status (confirmed/canceled) are useful but increase surface area and UI complexity.
2. A single global calendar is simpler, but per-user scheduling is the most common expectation and avoids accidental cross-user collisions.

### Decision 2: Time representation and time zones
Choice:
1. All times are stored and transmitted as UTC instants.
2. The UI displays times in the viewer’s local time zone.
3. start_time and end_time are inclusive-exclusive for overlap math, using the standard rule:
   new.start < existing.end AND new.end > existing.start

Rationale:
UTC storage avoids daylight savings issues in backend logic. Local display meets user expectations. Inclusive-exclusive boundaries reduce edge-case ambiguity where one appointment ends exactly when another begins.

### Decision 3: Required validations at create-time
Choice:
1. title must be non-empty after trimming
2. end_time must be strictly after start_time
3. duration must be reasonable (maximum 24 hours)

Rationale:
These are the smallest set of validations that prevent obviously invalid data and improve UX without over-specifying business rules.

### Decision 4: Supported operations for MVP
Choice:
1. Create appointment
2. List appointments within a requested time window
3. Delete appointment by id
4. Create recurring series (weekly only)
5. List occurrences within a requested time window
Not supported in MVP:
1. Update appointment
2. Cancel vs delete distinction
3. Invitees, multi-calendar sharing, reminders
4. Delete recurring series
5. Recurring exceptions (skip/override) via API

Rationale:
The requirements explicitly call out create, view, remove. Deferring update keeps the API smaller and avoids additional conflict scenarios (moving appointments).

### Decision 5: How appointments are displayed
Choice:
Primary view is a day schedule timeline:
1. Date navigation (previous/next, date picker)
2. Time grid with appointments rendered as blocks positioned by start/end time
3. Overlapping appointments are visually stacked or placed side-by-side
4. User switcher input (user_id) to scope the schedule
5. Create flow starts from selecting a time range (click or click-drag), then a modal to enter title and notes
6. Quick create via a "New" button that opens the create modal
7. Delete flow is started by selecting an existing appointment, then confirming in a modal
8. Recurrence is configured in the create modal (Once vs Weekly, interval, weekdays, ending rule, time zone display)

Rationale:
A day timeline is the simplest UI that still “makes sense for scheduling” because it immediately communicates availability and conflicts.

### Decision 6: Listing semantics and API shape
Choice:
ListAppointments requires:
1. user_id
2. window_start
3. window_end
The server returns appointments that intersect the window and sorts by start_time ascending.

Rationale:
Bounding list requests is essential for performance and aligns with day/week UI. Intersection semantics support appointments that start before the window but extend into it.

## Assumptions (Current)
1. Users are identified by a simple user_id string provided by the client for MVP.
2. The system does not require authentication for this assessment unless added later.
3. An appointment belongs to exactly one user_id.

## Technical Decisions

### Decision 7: Persistence and local provisioning
Choice:
1. Postgres is the source of truth.
2. Docker Compose provisions Postgres for local development.

Rationale:
Postgres provides strong transactional guarantees and a straightforward path to realistic concurrency behavior. Docker Compose keeps setup consistent across machines.

Trade-offs considered:
1. SQLite would be simpler to run, but is less representative of multi-user deployment scenarios.

### Decision 8: Migrations
Choice:
1. Use goose for schema migrations.
2. Migrations are applied explicitly via Makefile commands, not automatically on server startup.

Rationale:
Versioned migrations make schema evolution clear and reviewable. Avoiding automatic migration on startup reduces risk and makes environments more predictable.

### Decision 9: Command ergonomics
Choice:
Use a Makefile as the primary entrypoint for common workflows (generate, test, run, db up/down, migrate).

Rationale:
This standardizes commands for reviewers and reduces “it works on my machine” drift.

### Decision 10: Configuration management
Choice:
Use viper to load environment variables into a typed configuration used by the server.

Rationale:
Keeps configuration centralized and supports different environments without changing code.

### Decision 11: Reliability and shutdown behavior
Choice:
Implement graceful shutdown for the gRPC server:
1. Stop accepting new requests
2. Allow in-flight RPCs to finish within a timeout
3. Close DB connections cleanly

Rationale:
This prevents request truncation and avoids leaving connections in an unknown state during deploys or local restarts.

### Decision 12: Go project layout
Choice:
Adopt golang-standards/project-layout:
1. cmd/ for binaries
2. internal/ for application code
3. api/ for protobuf definitions

Rationale:
Familiar layout for Go reviewers and keeps boundaries clear.

### Decision 13: Testability and dependency boundaries
Choice:
1. Pass interfaces across package boundaries (service depends on repository interfaces).
2. Keep transports and stores as concrete implementations behind interfaces.

Rationale:
This keeps business logic testable without spinning up infrastructure and reduces coupling between layers.

### Decision 14: Database access layer
Choice:
Use Bun ORM (uptrace/bun) for database access.

Rationale:
Bun provides a productive query builder/ORM on top of Postgres while still allowing explicit SQL when needed. It keeps repository code readable without spreading SQL strings across the codebase.

Trade-offs considered:
1. database/sql with hand-written SQL is simpler and lower-level, but increases repetitive mapping code.
2. sqlc generates type-safe accessors, but requires additional workflow and is less flexible for evolving queries during an MVP.

### Decision 15: Overlap prevention at the database layer
Choice:
Use a Postgres exclusion constraint to prevent overlapping appointments per user_id:
1. EXCLUDE USING gist (user_id WITH =, tstzrange(start_time, end_time, '[)') WITH &&)
2. Enable the btree_gist extension so user_id equality can participate in the GiST index.

Interpretation (what constitutes a scheduling conflict):
1. Conflict scope is per user_id calendar only (no rooms/providers/resources in MVP).
2. Two appointments conflict when their time ranges overlap, using inclusive-exclusive bounds:
   new.start_time < existing.end_time AND new.end_time > existing.start_time
3. Appointments that touch at the boundary are allowed:
   existing ends at 10:00 and new starts at 10:00 is not a conflict.

Implementation notes:
1. The service validates basic fields and normalizes timestamps to UTC, then attempts the insert.
2. The repository maps the exclusion-constraint violation to a conflict error.
3. The gRPC layer returns codes.FailedPrecondition with a plain string message for conflicts.
4. The frontend maps FailedPrecondition to a "Time conflict" UI title and displays the server message when present.
5. Idempotency conflicts are also surfaced as codes.FailedPrecondition with a separate message.

Rationale:
This enforces the scheduling invariant under concurrency without relying solely on application-level checks. btree_gist is required because text equality does not have a default GiST operator class; the extension supplies GiST operator classes for btree-style operators like =, which makes the exclusion constraint valid.

Trade-offs considered:
1. App-level transactional overlap checks are simpler but are easier to get wrong under race conditions.
2. Dropping the user_id component avoids btree_gist but would incorrectly enforce a single global calendar.

### Decision 16: Strict consistency for concurrent bookings
Choice:
When multiple requests attempt to book overlapping time ranges for the same user_id, the system enforces strict consistency:
1. At most one request can succeed.
2. Conflicting requests are rejected with a conflict response.
3. Calendar writes for a user_id are serialized with a per-user advisory lock in the database.

Rationale:
Scheduling is trust-sensitive. Allowing double-booking and later reconciliation creates a poor experience and hard-to-reason-about state. Using Postgres as the source of truth provides correct behavior under concurrency without relying on application-level coordination.

User experience trade-offs:
1. The UI may occasionally show an available slot that becomes unavailable at submit-time due to a concurrent booking.
2. To keep the experience clear, the API returns conflict-specific feedback and the UI displays it as "Time conflict" with actionable guidance.

### Decision 17: Frontend tooling
Choice:
Use Vite to create and run the React + TypeScript frontend.

Rationale:
Vite provides fast local iteration and a simple, conventional React setup that reviewers can run easily.

### Decision 18: Frontend component library
Choice:
Use shadcn/ui as the component library (Radix UI primitives + Tailwind styling).

Rationale:
Provides high-quality, accessible UI primitives while keeping components in-repo and customizable, which fits a small product surface like scheduling.

### Decision 19: gRPC-Web proxy for browser clients
Choice:
1. Keep the backend as a standard gRPC server (grpc-go) on :50051.
2. Use Envoy as a gRPC-Web proxy for the browser, listening on :8080 and published locally on :8081 via Docker Compose.
3. The frontend uses a gRPC-Web compatible transport and targets the proxy URL (default http://localhost:8081) rather than the raw gRPC port.
4. The proxy base URL is configurable via VITE_GRPC_WEB_BASE_URL.

Rationale:
Browsers cannot call a raw gRPC server directly. Envoy’s grpc_web filter translates gRPC-Web requests into standard gRPC, letting us keep grpc-go on the backend without adding Connect-Go or changing the service implementation. Envoy is a production-grade proxy with a clear path to TLS, auth, and observability when needed.

Trade-offs considered:
1. A smaller dev-only proxy (grpcwebproxy) is simpler to run but provides less flexibility for production needs.
2. Adding Connect-Go enables the Connect protocol directly, but introduces another backend dependency and a different transport layer than gRPC.
3. Keeping a single Envoy config is simpler; having both a static config and an envsubst-templated config improves deploy flexibility at the cost of duplication.
4. Docker Compose uses the static Envoy config; the templated config is for containerized deployment.

### Decision 20: Recurring appointments model (calendar-grade)
Choice:
Represent recurring appointments as a recurrence rule plus exceptions:
1. Store a recurring series entity (rule, time zone, and template fields like title/notes and duration).
2. Store exceptions for individual occurrences (skip or override).
3. Generate occurrences for a requested time window at read-time and merge them with one-off appointments.
4. Current supported frequency is weekly only (interval + byweekday) with an end condition (until or count).
5. Occurrence ids are derived from the occurrence start timestamp (UTC) and represented as a string.

Rationale:
This matches how mature calendar products behave (edit single occurrence, edit series, skip dates) while keeping storage bounded for long-running series.

Trade-offs considered:
1. Materializing all occurrences makes conflict enforcement simple but makes edits and long series expensive.
2. Rule-based generation is more complex but is the most flexible long-term model.

### Decision 21: Conflict detection for recurring appointments
Choice:
Use pure rule-based conflict detection with transactional locking:
1. All calendar writes for a user_id run in a database transaction.
2. The transaction acquires a per-user_id lock to serialize calendar writes.
3. The service generates the affected occurrences in-scope and checks overlaps against existing one-off appointments and other generated occurrences.
4. Conflicts are rejected with FailedPrecondition and the "Time conflict" UI copy.
5. Conflict checks are bounded by a 180-day lookahead window from the series start_time (or earlier until), plus one occurrence duration.
6. Existing recurring series are expanded for the conflict window and merged with exceptions (skip/override) before overlap checks.

Rationale:
Rule-based conflict checks allow the system to return actionable feedback (which occurrence conflicts) without materializing an additional reservations table. Transactional locking prevents race conditions between concurrent writes for the same calendar.

User experience trade-offs:
1. Under high contention for a single user_id, writes may queue behind the lock (higher latency) but outcomes remain consistent and explainable.
2. Open-ended recurrence rules require a bounded evaluation window or an explicit end (count/until) to keep latency predictable.
3. To reduce edge-case misses, exception rows are queried with a ±14-day buffer around the conflict window.

### Decision 22: Default request timeouts for gRPC
Choice:
Set a default per-RPC timeout on the server when the client does not provide a deadline.
This is enforced via a unary interceptor and defaults to 10 seconds when not configured.

Rationale:
Prevents requests from hanging indefinitely under lock contention or slow downstream calls, and keeps tail latency bounded in multi-instance deployments.

### Decision 23: Per-instance database connection pool limits
Choice:
Configure and apply database/sql pool limits (max open/idle conns, connection lifetimes) via environment-backed config.
Defaults are applied in configuration and enforced on the connection pool at startup.

Rationale:
Avoids accidental connection storms when scaling the number of server instances.

### Decision 24: Idempotent create-appointment requests
Choice:
Support an Idempotency-Key request header for CreateAppointment:
1. The server derives a deterministic appointment id from (user_id, idempotency_key).
2. If a retry hits an existing id, return the existing appointment if the payload matches.
3. If the same key is reused with a different payload, reject the request.
4. The server accepts Idempotency-Key and X-Idempotency-Key and enforces a maximum key length (256 bytes).

Rationale:
Makes client retries safe across instances without requiring server affinity.

### Decision 25: Browser client RPC stack
Choice:
Use ConnectRPC's generated TypeScript clients and connect-web transport for the frontend:
1. buf generates TypeScript message and service definitions.
2. The frontend uses @connectrpc/connect and @connectrpc/connect-web to call the Envoy gRPC-Web proxy.

Rationale:
This keeps the browser client strongly typed from the protobuf schema with a small, maintained transport layer while still using grpc-go on the backend behind Envoy.

## Questions For Stakeholders (And How We Proceeded)
1. Is this a single shared calendar or per-user calendars
   Proceeded with per-user calendars because it is the most typical scheduling model.
2. Should appointments support updates and cancellations
   Proceeded with create/list/delete for one-off appointments only as explicitly required.
3. Do appointments need locations, attendees, or reminders
   Proceeded with title and optional notes only to keep scope focused.

## Developer Questions
1. If this service needed to run across multiple instances, what would change about the implementation?
   Keep servers stateless and rely on Postgres as the source of truth. Per-user calendar writes remain serialized with pg_advisory_xact_lock so correctness holds across instances. To make multi-instance operation safer, we added server-side request timeouts, configurable DB connection pool limits, and idempotent CreateAppointment handling via an Idempotency-Key header (Decisions 22–24). If we introduce read replicas, ensure any operation that writes or relies on locks is pinned to the primary. At higher scale, the heavier path will be read-time recurrence expansion; we’d likely add caching/materialization and/or partitioning by user_id if the database is sharded.

## If I Had More Time
1. Add update and cancel semantics with audit history.   
2. Add richer appointment fields (location, attendees) only if required by product.
3. Add better accessibility and keyboard-first scheduling flows in the UI.
4. Return structured conflict details (e.g., conflicting window and suggestions) to the UI.
5. Integrate Redis for distributed locking and caching to speed up conflict checks at higher throughput.
