# Distributed Job Processor

A production-ready distributed system for asynchronous job processing using NestJS, Go, RabbitMQ, and PostgreSQL. Deployable via Docker Compose (local dev) or Kubernetes with KEDA autoscaling (staging/production). Designed for extensibility and horizontal scalability.

## Architecture Overview

```
                        WRITE PATH
┌─────────────┐   HTTP    ┌─────────────────┐   AMQP   ┌──────────────────┐
│   Client    │──────────▶│   API (NestJS)  │─────────▶│  RabbitMQ        │
└─────────────┘           └────────┬────────┘          └────────┬─────────┘
                                   │ SQL INSERT                  │ consume
                                   ▼                             ▼
                          ┌─────────────────┐           ┌──────────────────┐
                          │   PostgreSQL    │◀──────────│  Worker (Go)     │
                          └────────┬────────┘ SQL UPDATE└────────┬─────────┘
                                   │                             │ /metrics
                        READ PATH  │                             ▼
┌─────────────┐  gRPC   ┌──────────┴──────┐           ┌──────────────────┐
│  Internal   │────────▶│ query-service   │           │  Prometheus       │
│  Service    │         │    (Go gRPC)    │           │  Grafana          │
└─────────────┘         └─────────────────┘           │  Tempo (traces)  │
                                                       │  Loki (logs)     │
                                                       └──────────────────┘
```

Write path: HTTP → RabbitMQ (async, via API). Read path: gRPC → PostgreSQL (sync, via query-service). Both paths share the same database; neither path depends on the other.

Distributed traces are propagated via W3C `traceparent` headers across the API→RabbitMQ→Worker boundary using OpenTelemetry, enabling end-to-end trace correlation in Grafana Tempo.

### Job Lifecycle

```
pending → processing → completed
                    ↘ failed (after max retries → dead letter queue)
```

## Project Structure

```
distributed-job-processor/
├── proto/
│   └── job.proto                 # Protobuf contract (source of truth for gRPC)
├── api/                          # NestJS HTTP API (write path)
│   └── src/
│       ├── jobs/
│       │   ├── domain/           # Entities, repository & broker ports
│       │   ├── application/      # Use cases, DTOs with per-type payload validation
│       │   └── infrastructure/   # TypeORM, RabbitMQ, HTTP controllers
│       └── shared/
│           ├── config/           # Database config
│           ├── filters/          # Global exception filter
│           └── guards/           # ApiKeyGuard, ThrottlerGuard (applied globally)
├── worker/                       # Go async worker
│   ├── cmd/worker/               # Entrypoint + config
│   └── internal/
│       ├── domain/               # Job entity, ports (interfaces)
│       ├── application/          # Handler registry + job handlers
│       │   ├── registry/         # Dynamic handler registry (Strategy pattern)
│       │   └── handlers/         # email, report, data-processing + InstrumentedHandler
│       ├── infrastructure/       # RabbitMQ consumer, PostgreSQL repo, metrics
│       └── pool/                 # Concurrent worker pool (Bulkhead pattern)
├── query-service/                # Go gRPC query service (read path)
│   ├── cmd/server/               # Entrypoint + wiring
│   ├── gen/job/                  # Generated protobuf Go code (do not edit)
│   └── internal/
│       ├── repository/           # PostgreSQL read-only repository
│       └── server/               # gRPC server implementation
├── k8s/                          # Kubernetes manifests
│   ├── namespace.yaml
│   ├── secrets/                  # app-secrets (postgres, rabbitmq, api-key)
│   ├── postgres/                 # StatefulSet + headless Service
│   ├── rabbitmq/                 # StatefulSet + headless Service
│   ├── api/                      # Deployment + Service + Ingress + HPA
│   ├── worker/                   # Deployment + KEDA ScaledObject
│   └── query-service/            # Deployment + Service
├── monitoring/
│   ├── prometheus/               # Scrape config
│   ├── grafana/                  # Datasources + pre-built dashboard
│   ├── tempo/                    # Distributed tracing backend (OTLP)
│   ├── loki/                     # Log aggregation
│   └── promtail/                 # Log collector (extracts trace_id as label)
├── Makefile                      # proto, cluster-create, images-build, deploy
├── .github/workflows/            # CI: lint + test + build + docker
└── docs/
    ├── adr/                      # Architecture Decision Records
    └── postman/                  # Postman collection
```

## Quick Start

### Option A — Docker Compose (local dev)

**Prerequisites:** Docker + Docker Compose

```bash
cp .env.example .env
docker compose up --build
```

| Service           | URL / Address                          |
|-------------------|----------------------------------------|
| API               | http://localhost:3000                  |
| Query Service     | localhost:50051 (gRPC)                 |
| RabbitMQ UI       | http://localhost:15672 (guest / guest) |
| Prometheus        | http://localhost:9091                  |
| Grafana           | http://localhost:3001 (admin / admin)  |
| Tempo (traces)    | http://localhost:3200                  |

### Option B — Kubernetes (k3d local cluster)

**Prerequisites:** Docker, [k3d](https://k3d.io), kubectl, helm

```bash
# 1. Create cluster + registry
make cluster-create

# 2. Build and push images to local registry
make images-build images-push

# 3. Deploy all manifests
make deploy

# 4. Port-forward for local access
make port-forward
```

| Service           | URL / Address              |
|-------------------|----------------------------|
| API               | http://127.0.0.1:3000      |
| Query Service     | 127.0.0.1:50051 (gRPC)    |

The worker scales automatically from 1 to 10 replicas based on RabbitMQ queue depth via KEDA (1 replica per 10 queued messages).

```bash
# Check cluster status
make status

# Tear down
make undeploy
make cluster-delete
```

## Authentication

All API endpoints require an `X-API-Key` header validated against the `API_KEY` environment variable. If `API_KEY` is not set, the guard is skipped (useful for local development without config).

```bash
# All requests must include:
-H "X-API-Key: your-api-key"
```

A missing or incorrect key returns `401 Unauthorized`.

## Manual Testing

Once all services are running, follow these steps to verify the full flow.

### 1. Create jobs

```bash
# email job
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"type":"email","payload":{"to":"user@example.com","subject":"Hello"},"maxAttempts":3}' | jq

# report job
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"type":"report","payload":{"report_type":"monthly"},"maxAttempts":3}' | jq

# data-processing job
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"type":"data-processing","payload":{"source":"s3://bucket/file.csv"},"maxAttempts":3}' | jq
```

Each response will have `"status": "pending"`. The worker processes it within milliseconds.

### 2. Check job status

```bash
curl -s http://localhost:3000/jobs/<id> \
  -H "X-API-Key: your-api-key" | jq
```

Expected: `"status": "completed"`, `"attempts": 1`.

### 3. List all jobs

```bash
curl -s "http://localhost:3000/jobs?limit=10&offset=0" \
  -H "X-API-Key: your-api-key" | jq
```

### 4. Verify payload validation (should return 400)

```bash
# invalid job type
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"type":"unknown-type","payload":{}}' | jq

# email job missing required field "to"
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"type":"email","payload":{"subject":"no recipient"}}' | jq
```

### 5. Check worker metrics

```bash
curl -s http://localhost:9090/metrics | grep worker_
```

Expected output:
```
worker_jobs_completed_total{job_type="email"} 1
worker_jobs_completed_total{job_type="report"} 1
worker_job_duration_seconds_count{job_type="email"} 1
worker_active_jobs 0
```

### 6. Check RabbitMQ

Open http://localhost:15672 (guest / guest) → Queues tab.
You should see `jobs` and `jobs.dead` queues with 0 messages ready.

### 7. Query jobs via gRPC

Once a job is created its ID can be queried directly through the gRPC query-service. Requires [grpcurl](https://github.com/fullstorydev/grpcurl).

```bash
# list available services (uses server reflection — no .proto file needed)
grpcurl -plaintext localhost:50051 list

# get a specific job
grpcurl -plaintext -d '{"id": "<job-id>"}' localhost:50051 job.v1.JobService/GetJob

# list jobs with pagination
grpcurl -plaintext -d '{"limit": 10, "offset": 0}' localhost:50051 job.v1.JobService/ListJobs
```

Expected `GetJob` response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "email",
  "status": "completed",
  "attempts": 1,
  "maxAttempts": 3,
  "createdAt": "2024-04-26T10:00:00Z"
}
```

gRPC error codes: `NOT_FOUND` (job does not exist), `INVALID_ARGUMENT` (missing id), `INTERNAL` (infrastructure failure).

### 8. Check Grafana dashboard

Open http://localhost:3001 (admin / admin) → Dashboards → Job Processor - Worker.
The dashboard shows completed/failed jobs per minute, p95 latency, and active jobs.

## Running Tests

```bash
# API — unit tests
cd api && npm test

# API — with coverage report
cd api && npm run test:cov

# Worker — unit tests with race detector
cd worker && go test ./... -race -v
```

## API Reference

### Authentication

All endpoints require the `X-API-Key` header:

```
X-API-Key: your-api-key
```

### Create a job

```
POST /jobs
Content-Type: application/json
X-API-Key: your-api-key
```

```json
{
  "type": "email",
  "payload": {
    "to": "user@example.com",
    "subject": "Welcome"
  },
  "maxAttempts": 3
}
```

**Response 201**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "email",
  "status": "pending",
  "attempts": 0,
  "maxAttempts": 3,
  "createdAt": "2024-04-26T10:00:00Z"
}
```

### Get job status

```
GET /jobs/:id
X-API-Key: your-api-key
```

### List jobs

```
GET /jobs?limit=20&offset=0
X-API-Key: your-api-key
```

## Supported Job Types

| Type              | Required payload fields |
|-------------------|-------------------------|
| `email`           | `to` (email string)     |
| `report`          | `report_type` (string)  |
| `data-processing` | `source` (string)       |

Payload shape is validated at the API boundary using per-type DTOs (`EmailPayloadDto`, `ReportPayloadDto`, `DataProcessingPayloadDto`). Invalid payloads are rejected with `400` before reaching the queue.

## Adding a New Job Type

**1.** Create a handler in `worker/internal/application/handlers/`:

```go
type MyHandler struct{ log *logger.Logger }

func (h *MyHandler) Handle(ctx context.Context, job *domain.Job) error {
    // your logic
    return nil
}
```

**2.** Register it in `worker/cmd/worker/main.go`:

```go
reg.Register("my-new-type", handlers.NewMyHandler(log))
```

**3.** Add a payload DTO and register the type in `api/src/jobs/application/create-job.dto.ts`:

```ts
export class MyNewTypePayloadDto {
  @IsString() @IsNotEmpty() requiredField!: string;
}

const VALID_JOB_TYPES = ['email', 'report', 'data-processing', 'my-new-type'] as const;

// add to resolvePayloadType map:
'my-new-type': MyNewTypePayloadDto,
```

No other changes required — the architecture is closed for modification, open for extension.

## Configuration

### API environment variables

| Variable                       | Description                          | Default  |
|--------------------------------|--------------------------------------|----------|
| `PORT`                         | HTTP port                            | `3000`   |
| `API_KEY`                      | API key required in X-API-Key header | —        |
| `POSTGRES_DSN`                 | PostgreSQL connection URL            | required |
| `RABBITMQ_URL`                 | RabbitMQ AMQP URL                    | required |
| `OTEL_SERVICE_NAME`            | Service name in traces               | `api`    |
| `OTEL_EXPORTER_OTLP_ENDPOINT`  | Tempo OTLP HTTP endpoint             | —        |

### Worker environment variables

| Variable                       | Description                   | Default  |
|--------------------------------|-------------------------------|----------|
| `POSTGRES_DSN`                 | PostgreSQL connection URL     | required |
| `RABBITMQ_URL`                 | RabbitMQ AMQP URL             | required |
| `WORKER_CONCURRENCY`           | Number of parallel goroutines | `5`      |
| `JOB_TIMEOUT_SECONDS`          | Max processing time per job   | `30`     |
| `METRICS_PORT`                 | Prometheus metrics port       | `9090`   |
| `OTEL_SERVICE_NAME`            | Service name in traces        | `worker` |
| `OTEL_EXPORTER_OTLP_ENDPOINT`  | Tempo OTLP HTTP endpoint      | —        |

### Query Service environment variables

| Variable       | Description               | Default  |
|----------------|---------------------------|----------|
| `POSTGRES_DSN` | PostgreSQL connection URL | required |
| `GRPC_PORT`    | gRPC server port          | `50051`  |

## Observability

### Metrics (Prometheus + Grafana)

The worker exposes Prometheus metrics at `:9090/metrics`:

| Metric                        | Type      | Description                    |
|-------------------------------|-----------|--------------------------------|
| `worker_jobs_completed_total` | Counter   | Jobs completed, by type        |
| `worker_jobs_failed_total`    | Counter   | Jobs failed, by type           |
| `worker_job_duration_seconds` | Histogram | Processing duration, by type   |
| `worker_active_jobs`          | Gauge     | Currently processing jobs      |

Grafana dashboard is auto-provisioned at startup (Docker Compose only).

### Distributed Tracing (OpenTelemetry + Tempo)

Both the API and worker are instrumented with OpenTelemetry. Traces are exported to Tempo via OTLP HTTP.

The W3C `traceparent` header is injected into AMQP message headers when the API publishes a job. The worker extracts it and continues the same trace, so a single trace spans the full HTTP → queue → worker flow.

Worker logs include `trace_id` for correlation with Loki:

```json
{"level":"INFO","msg":"job completed","job_id":"...","trace_id":"41d1423e786b75983feda80c3b2fc4f9"}
```

Set `OTEL_EXPORTER_OTLP_ENDPOINT` to enable trace export. If unset, W3C context propagation still works (trace IDs appear in logs) but spans are not exported.

## gRPC Reference

The query-service exposes a gRPC server defined by `proto/job.proto`. It is the **source of truth** — regenerate Go code with `make proto` whenever the contract changes.

### GetJob

```
rpc GetJob(GetJobRequest) returns (JobResponse)
```

| Field | Type   | Required |
|-------|--------|----------|
| `id`  | string | yes (UUID format) |

### ListJobs

```
rpc ListJobs(ListJobsRequest) returns (ListJobsResponse)
```

| Field    | Type  | Default |
|----------|-------|---------|
| `limit`  | int32 | `20`    |
| `offset` | int32 | `0`     |

### Error codes

| gRPC status        | Meaning                          |
|--------------------|----------------------------------|
| `NOT_FOUND`        | Job does not exist               |
| `INVALID_ARGUMENT` | Required field missing or empty  |
| `INTERNAL`         | Infrastructure failure           |

## Architecture Decision Records

### ADR-001 — Hexagonal Architecture over strict Clean Architecture

**Context:** The system needs to support swapping the message broker from RabbitMQ (v1) to Kafka (v2) without touching business logic.

**Decision:** Ports & Adapters (Hexagonal). Domain defines interfaces (`MessageConsumer`, `JobRepository`). Infrastructure implements them.

**Consequence:** Broker migration = new adapter implementing the existing port. Zero domain changes.

---

### ADR-002 — Go for the Worker

**Context:** The worker is I/O-bound with controlled concurrency requirements.

**Decision:** Go. Native goroutines, lightweight concurrency primitives (`sync.WaitGroup`, channels, semaphore pattern), and single static binary for deployment.

**Consequence:** Worker pool with configurable concurrency (`WORKER_CONCURRENCY`) without external thread pool libraries.

---

### ADR-003 — RabbitMQ over Kafka for v1

**Context:** v1 scope is a single worker with moderate throughput. Kafka's operational overhead (Zookeeper/KRaft, partition management) is not justified yet.

**Decision:** RabbitMQ with a dead letter exchange. At-least-once delivery + manual ack gives sufficient reliability guarantees.

**Consequence:** When throughput requirements grow, migrating to Kafka means implementing `KafkaConsumer` satisfying the `MessageConsumer` port and updating `main.go` wiring.

---

### ADR-004 — PostgreSQL as job store

**Context:** Jobs need durable state, queryability by status/type, and idempotency guarantees.

**Decision:** PostgreSQL with a UUID primary key. The job `id` is generated by the API before publishing, making the worker idempotent by design — re-processing the same message produces the same `UPDATE`.

**Consequence:** No distributed transaction needed between queue publish and DB write. The worst case is a duplicate `UPDATE` with the same data.

---

### ADR-005 — API key over JWT for service authentication

**Context:** The API needs protection against unauthorized access. The primary consumers are internal services and developer tooling, not end users with sessions.

**Decision:** Static API key validated via `X-API-Key` header, enforced by a global NestJS guard. JWT adds key rotation complexity and a token validation step that is not justified for service-to-service communication at this stage.

**Consequence:** Key rotation requires redeploying with a new `API_KEY` env var. Acceptable for v1; upgrading to JWT means replacing `ApiKeyGuard` with a Passport strategy — no controller changes required.

### ADR-006 — Separate Go gRPC service for the read path

**Context:** The system needs an internal service-to-service query interface with typed contracts and better performance than REST for reads. The API already handles the write path.

**Decision:** A dedicated Go microservice (`query-service`) exposing a gRPC server defined by `proto/job.proto`. The contract lives outside any single service so any team can consume it independently. The service connects only to PostgreSQL — it has no dependency on RabbitMQ, so a broker failure does not affect reads.

**Consequence:** Write path (HTTP → RabbitMQ → Worker) and read path (gRPC → PostgreSQL) are independently deployable and scalable. Adding a new query method = update the proto, regenerate, implement the method. No changes to the API or worker.

---

## Roadmap

- **v2 — Kafka adapter:** Implement `KafkaConsumer` using `confluent-kafka-go`. Consumer groups replace the worker pool for partition-level parallelism. Exactly-once semantics via idempotent producers.
- **Webhooks:** Callback URL per job for push-based status notification.
- **Job scheduling:** Delayed jobs via RabbitMQ TTL + dead-letter routing.
