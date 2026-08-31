# Khātere Backend

A private, consent-based photo-sharing API ("anti-Instagram") — written in raw Go, built around Clean/Hexagonal Architecture. This README describes the system as it currently stands.

## Core idea

Users create **Events** (e.g. a trip, a party). They tag other users into the Event. A tagged user must **approve** the tag before they become a member — nothing about them appears anywhere until they've consented. Approved members can upload **Photos** to the Event and see everyone else's. Removing yourself from an Event removes your visibility into it. This consent-first model (404, not 403, for anything you're not approved into) is the central design constraint that shapes most of the domain logic.

## Architecture

Clean/Hexagonal Architecture throughout:

```
internal/
  domain/        — entities, ports (interfaces), no framework code, no SQL
    event/
    photo/
    user/
    notification/
    gallery/
  application/   — use cases. Orchestrate domain ports. One file/type per use case.
    event/
    photo/
    user/
    notification/
    gallery/
  adapters/      — implementations of domain ports
    postgres/    — SQL implementations of each domain Repository
    http/        — HTTP handlers (translate HTTP <-> use case input/output)
    filesystem/  — local-disk Photo Storage adapter
    s3/          — Minio/S3 Photo Storage adapter (production)
    redis/       — caching decorators + Kafka-adjacent glue
    kafka/       — Kafka producers, consumers, outbox publisher
    thumbnailqueue/ — bounded worker pool for thumbnail generation
  middleware/    — auth, rate limiting
  telemetry/     — Prometheus metrics, OpenTelemetry tracing helpers
main.go          — composition root: wires every adapter into every use case
```

**Rule of dependency:** `domain` depends on nothing. `application` depends only on `domain` ports. `adapters` implement `domain` ports and may depend on anything. `main.go` is the only place that knows about concrete adapter types — everywhere else works through interfaces.

## Domains

| Domain | Responsibility |
|---|---|
| **User** | Registration, login (JWT), search, profile lookup |
| **Event** | Create/list/get events, tag/approve/reject/remove members |
| **Photo** | Upload photos to an Event, list an Event's photos with thumbnails |
| **Notification** | List/mark-read notifications (populated by the Kafka consumer, not written directly by other domains) |
| **Gallery** | Read-only projection: "which Events am I in, with a photo count" — no business rules of its own |

## Data layer

- **Postgres** — primary store for users, events, event_members, photos, notifications, and the outbox table.
- **Redis** — two caching decorators wrapping the Event and Gallery repositories (`adapters/redis/*_repository_cache.go`):
  - Event membership checks (`IsApprovedMember`) — 60s TTL, invalidated on every membership-changing write (add/approve/reject/remove).
  - Gallery reads — 30s TTL, no invalidation (short TTL alone is fine; Gallery tolerates staleness).
  - Also used for login rate limiting (`middleware/ratelimit.go` — 5 attempts / 15 min / IP, fixed window via `INCR` + `EXPIRE`).
- **Object storage (Minio/S3)** — `domain/photo.Storage` is a 3-method port (`Save`, `EnsureThumbnail`, `PublicURL`). `Save`/`EnsureThumbnail` return storage *keys*; `PublicURL` resolves a key into a real, time-limited fetchable URL. In production this is Minio, accessed with **presigned GET URLs** (15-minute expiry) — the bucket itself is private, since photo access must be gated on Event membership, not on public bucket ACLs. Switchable via `STORAGE_BACKEND=filesystem|s3`.

## Concurrency

- **`GetEventUseCase`** runs its two independent reads — the member list and the photo list — concurrently via `sync.WaitGroup`, instead of sequentially. Both are children of the same tracing span, so the parallelism is visible (and provable) in Jaeger, not just asserted in comments.
- **Thumbnail generation** runs in a bounded worker pool (`adapters/thumbnailqueue`), not inline on the upload request. `UploadPhotoUseCase` enqueues a job and returns immediately; a fixed number of workers (env-configurable) drain the queue. `ListEventPhotosUseCase` still calls `EnsureThumbnail` synchronously as an idempotent fallback, in case a photo is viewed before its background job finishes.
- **Graceful shutdown**: on `SIGINT`/`SIGTERM`, the server stops accepting new HTTP requests, then drains the thumbnail pool, then stops the outbox publisher and Kafka consumer, then closes Redis, then Postgres, then flushes OpenTelemetry — in that order.

## Event-driven notifications

Membership changes (tag / approve / reject) do **not** write notifications directly. Instead:

1. `EventRepository.AddMember` / `ApproveMember` / `RejectMember` each run inside a single Postgres transaction that **also** inserts a row into an `outbox` table — the membership change and the "an event happened" record are atomic. Either both happen or neither does.
2. A background **`OutboxPublisher`** polls the `outbox` table (`FOR UPDATE SKIP LOCKED`, batched, every 500ms), publishes each unpublished row to the matching Kafka topic, then marks it published — all within one transaction per batch. Delivery is at-least-once: consumers must tolerate occasional duplicate messages.
3. A **`NotificationConsumer`** reads from all four Kafka topics (`member-tagged`, `member-approved`, `member-rejected`, `photo-uploaded`) and writes the corresponding `notifications` rows.

This closes a real correctness gap that existed earlier: previously, the membership write and the notification publish were two separate, non-atomic steps — a crash between them silently dropped the notification with no membership-side error. They're now transactionally linked.

**Note on `photo_uploaded`:** this event does *not* go through the outbox — it's published directly from `UploadPhotoUseCase` after the photo row is committed. The Photo domain publishes the bare fact ("a photo was uploaded"); the consumer decides who cares by looking up the Event's approved members itself. This keeps Photo decoupled from Event membership logic. This asymmetry (outbox for membership events, direct publish for photo events) is a known, intentional gap — not yet unified.

## Observability

- **Metrics (Prometheus, `/metrics`)** — request count and latency per route (fixed route-pattern labels, not raw paths, to avoid cardinality blowup), thumbnail queue depth (`GaugeFunc`, read at scrape time), cache hit/miss counts per cache name.
- **Dashboards (Grafana)** — auto-provisioned on startup (datasource + one dashboard, both as files under `grafana/provisioning/`), five panels: request rate, p95 latency, error rate, thumbnail queue depth, cache hit rate.
- **Tracing (OpenTelemetry + Jaeger)** — every application-layer use case starts its own span (`telemetry.Tracer()`), not just the HTTP layer. This is what makes the `GetEventUseCase` concurrency claim verifiable rather than assumed: the two child spans for `ListMembers` and `ListPhotos` show overlapping start/end times in the trace.

## Local development

```bash
docker compose up -d
```

Brings up: the app, Postgres, Redis, Minio (+ console on :9001), Kafka (KRaft mode, no Zookeeper), Prometheus, Grafana, Jaeger.

| Service | URL |
|---|---|
| API | http://localhost:8080 |
| Jaeger UI | http://localhost:16686 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |
| Minio console | http://localhost:9001 |

`sweep.sh` exercises every endpoint end-to-end (Auth → User → Event → Notification → Photo → Gallery) against fresh, randomly-suffixed users each run — safe to re-run any number of times with no manual cleanup. Useful after any change, to confirm nothing broke and to generate real traffic for Jaeger/Grafana to show.

## Known gaps / not yet done

- `kafkaadapter.EventNotifier` and `domain/event.Notifier` are dead code as of the outbox migration — nothing calls them anymore. Not yet deleted.
- `photo_uploaded` events bypass the outbox (see above) — not yet unified with the membership-event pattern.
- No reverse proxy / TLS in front of Minio yet — presigned URLs currently use the internal Docker hostname, which only resolves inside the Compose network. Needed before any real external deployment.
- No backup strategy yet for the `minio_data` volume — deferred until a second storage location exists.
- Single-node Kafka (`replication-factor=1` everywhere) — fine for local dev, would need revisiting for a real multi-broker deployment.
