# Khātere Backend

A private, consent-based photo-sharing API ("anti-Instagram") — written in raw Go, built around Clean/Hexagonal Architecture. Users create Events, tag other users into them, and nothing about a tag is visible to anyone until the tagged person explicitly approves it. Approved members can upload and see Photos for that Event.

## Table of contents

- [Architecture](#architecture)
- [Domains](#domains)
- [Design patterns used, and why](#design-patterns-used-and-why)
- [Data layer](#data-layer)
- [Concurrency](#concurrency)
- [Event-driven notifications](#event-driven-notifications)
- [Observability](#observability)
- [Performance & profiling](#performance--profiling)
- [Local development](#local-development)
- [Known gaps / deliberately deferred](#known-gaps--deliberately-deferred)

## Architecture

Clean/Hexagonal Architecture throughout — domain logic has zero framework or SQL dependencies; everything talks through interfaces (**ports**), and concrete technology choices live only in **adapters**.

```
internal/
  domain/        — entities, ports (interfaces). No SQL, no HTTP, no framework code.
    event/  photo/  user/  notification/  gallery/
  application/   — use cases. One type per use case, orchestrates domain ports only.
    event/  photo/  user/  notification/  gallery/
  adapters/      — concrete implementations of domain ports
    postgres/       — SQL implementations of each Repository
    http/           — HTTP handlers (HTTP <-> use case translation only)
    filesystem/     — local-disk Photo Storage (dev)
    s3/             — Minio/S3 Photo Storage (production)
    redis/          — caching decorators for two repositories
    kafka/          — producers, consumer, outbox publisher
    thumbnailqueue/ — bounded worker pool for thumbnail generation
  middleware/    — JWT auth, login rate limiting
  telemetry/     — Prometheus metrics, OpenTelemetry tracing helpers
  imaging/       — pure image decode/resize/encode (no I/O of its own)
  db.go          — Postgres connection setup
main.go          — composition root: the only file that wires concrete adapters together
```

**Dependency rule:** `domain` depends on nothing else in this codebase. `application` depends only on `domain` ports. `adapters` implement those ports and may depend on anything. `main.go` is the only place that ever imports a concrete adapter type directly — every other file works entirely through interfaces.

## Domains

| Domain | Responsibility |
|---|---|
| **User** | Registration, JWT login, search, profile lookup |
| **Event** | Create/list/get events; tag, approve, reject, remove members |
| **Photo** | Upload photos to an Event; list an Event's photos with thumbnails |
| **Notification** | List / mark-read notifications — populated entirely by the Kafka consumer, never written directly by another domain |
| **Gallery** | Read-only projection: "which Events am I in, with a photo count." No business rules of its own — a pure query shape |

## Design patterns used, and why

- **Ports & Adapters (Hexagonal Architecture)** — every external dependency (DB, cache, object storage, message broker) is behind a small interface defined in `domain/`. This is why `STORAGE_BACKEND=filesystem|s3` is a one-line env-var switch, not a rewrite.
- **Decorator** — `adapters/redis/event_repository_cache.go` and `gallery_repository_cache.go` wrap the real Postgres repository and transparently add caching. The use-case layer has no idea caching exists; it just calls `Repository.IsApprovedMember(...)` like normal.
- **Repository** — one interface per aggregate (`domainevent.Repository`, `domainphoto.Repository`, etc.), one Postgres implementation each, swappable in tests or behind a cache without touching business logic.
- **Composition root / manual dependency injection** — `main.go` is the single place every concrete type gets constructed and wired together. No DI framework, no magic — just explicit constructor calls, which makes the entire dependency graph visible by reading one file top to bottom.
- **Bounded worker pool** — `adapters/thumbnailqueue/pool.go`: a fixed number of goroutines draining a fixed-size channel, so thumbnail generation can't spawn unbounded goroutines under load, and back-pressures (blocks) once the queue is full instead of silently dropping work.
- **Transactional outbox** — `adapters/postgres/event_repo.go` + `adapters/kafka/outbox_publisher.go`: a membership change and its corresponding "this happened" event are written in one Postgres transaction, so a crash between "update the DB" and "tell Kafka" can no longer happen. A separate poller later delivers the staged event, at-least-once.
- **Presigned URLs for private object storage** — the S3/Minio bucket is never public; `domainphoto.Storage.PublicURL` mints a short-lived (15 min), signed URL only after an Event-membership check has already passed. Access control lives in the app, not the bucket ACL.
- **404-not-403 privacy model** — anything you're not an approved member of returns "not found," never "forbidden." This avoids leaking the *existence* of an Event or a photo to someone who isn't supposed to know it exists at all.

## Data layer

- **Postgres** — primary store: users, events, event_members, photos, notifications, outbox.
- **Redis** — two caching decorators (membership checks: 60s TTL, invalidated on every membership write; Gallery reads: 30s TTL, no invalidation needed) plus login rate limiting (5 attempts / 15 min / IP, fixed window).
- **Object storage (Minio/S3)** — `domain/photo.Storage` is a 3-method port (`Save`, `EnsureThumbnail`, `PublicURL`). The first two deal in storage *keys*; only `PublicURL` resolves a key into a real, time-limited, fetchable URL. Switchable via `STORAGE_BACKEND=filesystem|s3`.

## Concurrency

- **`GetEventUseCase`** runs its member-list read and photo-list read concurrently (`sync.WaitGroup`), instead of sequentially — verified for real by checking the OpenTelemetry trace: both child spans have overlapping start/end timestamps, not sequential ones.
- **Thumbnail generation** runs in the bounded worker pool above, not inline on the upload request — `UploadPhotoUseCase` enqueues and returns immediately. `ListEventPhotosUseCase` still calls the same resize function synchronously as an idempotent fallback, in case a photo is viewed before its background job finishes.
- **Graceful shutdown** — on `SIGINT`/`SIGTERM`: stop accepting new HTTP requests → drain the thumbnail pool → stop the outbox publisher and Kafka consumer → close Redis → close Postgres → flush OpenTelemetry, strictly in that order.

## Event-driven notifications

Membership changes (tag / approve / reject) never write a notification directly. Instead:

1. `EventRepository.AddMember` / `ApproveMember` / `RejectMember` each run inside one Postgres transaction that also inserts a row into an `outbox` table — the membership change and "an event happened" are atomic.
2. A background **`OutboxPublisher`** polls `outbox` (`SELECT ... FOR UPDATE SKIP LOCKED`, batched, every 500ms), publishes each unpublished row to Kafka, then marks it published — one transaction per batch. Delivery is at-least-once by design; consumers must tolerate occasional duplicates.
3. A **`NotificationConsumer`** reads all four topics (`member-tagged`, `member-approved`, `member-rejected`, `photo-uploaded`) and writes the actual `notifications` rows.

`photo_uploaded` is the one exception: it's published directly from `UploadPhotoUseCase` (not through the outbox), and the consumer — not the Photo domain — decides who gets notified by looking up the Event's approved members itself. This keeps the Photo domain fully decoupled from Event membership rules.

## Observability

- **Metrics (`/metrics`, Prometheus)** — request count and latency per route (fixed route-pattern labels, never raw paths, to avoid cardinality blowup), thumbnail queue depth, cache hit/miss counts per cache.
- **Dashboards (Grafana)** — auto-provisioned from files under `grafana/provisioning/`: five panels (request rate, p95 latency, error rate, queue depth, cache hit rate).
- **Tracing (OpenTelemetry + Jaeger)** — every application-layer use case starts its own span, not just the HTTP layer, so claims like "these two reads run concurrently" are provable in a trace waterfall, not just asserted in a comment.

## Performance & profiling

- **pprof** runs on a separate, non-public port (`127.0.0.1:6060`, never exposed in `compose.yaml`) — reachable only via `docker compose exec`, deliberately kept off the public internet since it exposes goroutine stacks and memory internals.
- **Postgres connection pool** is explicitly bounded (`SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime`) — the default Go `*sql.DB` pool is unbounded, which under real load can exhaust Postgres's own connection limit and cause cascading failures across every domain sharing that one `*sql.DB`.
- **`scripts/`** — load-test and profiling helpers used to validate the above with real evidence rather than guesses:
  - `load_test_reads.sh` — `hey`-based load against the two Redis-cached read paths.
  - `load_test_uploads.sh` — round-robin multipart photo uploads across several pre-created events (avoids the per-Event photo cap, avoids `hey`'s lack of multipart support).
  - `capture_profiles.sh` — runs an upload load and captures CPU/heap/goroutine pprof snapshots at the same time, saved under a timestamped `profiles/` folder.
- One real finding from using these: a manual, naive nested-loop image resize (`internal/imaging/resize.go`) was suspected as a likely CPU hotspot going in. Profiling under real load disproved that — it never appeared meaningfully in a CPU profile. The actual cost under upload load is dominated by Go's multipart form buffering and S3 request-chunk signing, not the resize step. Nothing was "optimized" on the strength of a guess; the guess was checked first, and it was wrong.

## Local development

```bash
docker compose up -d
```

Brings up: the app, Postgres, Redis, Minio (+ console on `:9001`), Kafka (KRaft mode, no Zookeeper), Prometheus, Grafana, Jaeger.

| Service | URL |
|---|---|
| API | http://localhost:8080 |
| Jaeger UI | http://localhost:16686 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |
| Minio console | http://localhost:9001 |

`sweep.sh` exercises every endpoint end-to-end (Auth → User → Event → Notification → Photo → Gallery) against fresh, randomly-suffixed users each run — safe to re-run any number of times with no manual cleanup.

## Known gaps / deliberately deferred

- `kafkaadapter.EventNotifier` and `domain/event.Notifier` are dead code since the outbox migration replaced their only call sites — not yet deleted.
- `photo_uploaded` bypasses the outbox (see above) — not yet unified with the membership-event pattern.
- No reverse proxy / TLS in front of Minio — presigned URLs currently use the internal Docker hostname, which only resolves inside the Compose network. Needed before any real external deployment; deliberately deferred until an actual server/domain exists to configure it against.
- No backup strategy for the `minio_data` volume yet — deferred until a second storage location exists to back up to.
- Single-node Kafka (`replication-factor=1` everywhere) — fine for local dev, would need revisiting for a real multi-broker deployment.
- No Microservices split (the codebase is structured so it *could* be split along its existing domain boundaries later, but this was explicitly scoped out for now).
- Performance profiling infrastructure exists and has been used to validate real findings, but no tuning has been done on the strength of it — deliberately not chasing optimization without a concrete reason to believe it's needed yet.
