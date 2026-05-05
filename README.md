# Distributed Job Processor

A production-ready distributed system for asynchronous job processing using NestJS, Go, RabbitMQ, and PostgreSQL. Designed for extensibility and horizontal scalability.

## Architecture Overview

```
┌─────────────┐     HTTP      ┌─────────────────┐     AMQP      ┌──────────────────┐
│   Client    │──────────────▶│   API (NestJS)  │──────────────▶│  RabbitMQ        │
└─────────────┘               └────────┬────────┘               └────────┬─────────┘
                                        │                                  │
                                        │ SQL                              │ consume
                                        ▼                                  ▼
                               ┌─────────────────┐               ┌──────────────────┐
                               │   PostgreSQL    │◀──────────────│  Worker (Go)     │
                               └─────────────────┘   SQL update  └────────┬─────────┘
                                                                           │ /metrics
                                                                           ▼
                                                                  ┌──────────────────┐
                                                                  │  Prometheus      │
                                                                  │  Grafana         │
                                                                  └──────────────────┘
```

### Job Lifecycle

```
pending → processing → completed
                    ↘ failed (after max retries → dead letter queue)
```

## Project Structure

```
distributed-job-processor/
├── api/                          # NestJS HTTP API
│   └── src/
│       ├── jobs/
│       │   ├── domain/           # Entities, repository & broker ports
│       │   ├── application/      # Use cases, DTOs
│       │   └── infrastructure/   # TypeORM, RabbitMQ, HTTP controllers
│       └── shared/               # Global config, exception filters
├── worker/                       # Go async worker
│   ├── cmd/worker/               # Entrypoint + config
│   └── internal/
│       ├── domain/               # Job entity, ports (interfaces)
│       ├── application/          # Handler registry + job handlers
│       │   ├── registry/         # Dynamic handler registry
│       │   └── handlers/         # email, report, data-processing
│       ├── infrastructure/       # RabbitMQ consumer, PostgreSQL repo, metrics
│       └── pool/                 # Concurrent worker pool
├── monitoring/
│   ├── prometheus/               # Scrape config
│   └── grafana/                  # Datasources + pre-built dashboard
├── .github/workflows/            # CI: lint + test + build + docker
└── docs/adr/                     # Architecture Decision Records
```

## Quick Start

### Prerequisites

- Docker + Docker Compose

### Run

```bash
docker compose up --build
```

| Service       | URL                              |
|---------------|----------------------------------|
| API           | http://localhost:3000            |
| RabbitMQ UI   | http://localhost:15672 (guest/guest) |
| Prometheus    | http://localhost:9091            |
| Grafana       | http://localhost:3001 (admin/admin) |

## Manual Testing

Once all services are running, follow these steps to verify the full flow.

### 1. Create jobs

```bash
# email job
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -d '{"type":"email","payload":{"to":"user@example.com","subject":"Hello"},"maxAttempts":3}' | jq

# report job
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -d '{"type":"report","payload":{"report_type":"monthly"},"maxAttempts":3}' | jq

# data-processing job
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -d '{"type":"data-processing","payload":{"source":"s3://bucket/file.csv"},"maxAttempts":3}' | jq
```

Each response will have `"status": "pending"`. The worker processes it within milliseconds.

### 2. Check job status

```bash
# replace <id> with the id from the previous response
curl -s http://localhost:3000/jobs/<id> | jq
```

Expected: `"status": "completed"`, `"attempts": 1`.

### 3. List all jobs

```bash
curl -s "http://localhost:3000/jobs?limit=10&offset=0" | jq
```

### 4. Verify validation (should return 400)

```bash
curl -s -X POST http://localhost:3000/jobs \
  -H "Content-Type: application/json" \
  -d '{"type":"unknown-type","payload":{}}' | jq
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

### 7. Check Grafana dashboard

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

### Create a job

```bash
POST /jobs
Content-Type: application/json

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

```bash
GET /jobs/:id
```

### List jobs

```bash
GET /jobs?limit=20&offset=0
```

## Supported Job Types

| Type              | Required payload fields         |
|-------------------|---------------------------------|
| `email`           | `to` (string)                   |
| `report`          | `report_type` (string)          |
| `data-processing` | `source` (string)               |

## Adding a New Job Type

1. Create a handler in `worker/internal/application/handlers/`:

```go
type MyHandler struct{ log *logger.Logger }

func (h *MyHandler) Handle(ctx context.Context, job *domain.Job) error {
    // your logic
    return nil
}
```

2. Register it in `worker/cmd/worker/main.go`:

```go
reg.Register("my-new-type", handlers.NewMyHandler(log))
```

3. Add the type to the allowed list in `api/src/jobs/application/create-job.dto.ts`:

```ts
const VALID_JOB_TYPES = ['email', 'report', 'data-processing', 'my-new-type'] as const;
```

No other changes required — the architecture is closed for modification, open for extension.

## Configuration

### API environment variables

| Variable       | Description              | Default     |
|----------------|--------------------------|-------------|
| `PORT`         | HTTP port                | `3000`      |
| `POSTGRES_DSN` | PostgreSQL connection URL | required   |
| `RABBITMQ_URL` | RabbitMQ AMQP URL        | required    |

### Worker environment variables

| Variable              | Description                      | Default |
|-----------------------|----------------------------------|---------|
| `POSTGRES_DSN`        | PostgreSQL connection URL        | required |
| `RABBITMQ_URL`        | RabbitMQ AMQP URL                | required |
| `WORKER_CONCURRENCY`  | Number of parallel goroutines    | `5`      |
| `JOB_TIMEOUT_SECONDS` | Max processing time per job      | `30`     |
| `METRICS_PORT`        | Prometheus metrics port          | `9090`   |

## Observability

The worker exposes Prometheus metrics at `:9090/metrics`:

| Metric                             | Type      | Description                        |
|------------------------------------|-----------|------------------------------------|
| `worker_jobs_completed_total`      | Counter   | Jobs completed, by type            |
| `worker_jobs_failed_total`         | Counter   | Jobs failed, by type               |
| `worker_job_duration_seconds`      | Histogram | Processing duration, by type       |
| `worker_active_jobs`               | Gauge     | Currently processing jobs          |

Grafana dashboard is auto-provisioned at startup.

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

## Roadmap

- **v2 — Kafka adapter:** Implement `KafkaConsumer` using `confluent-kafka-go`. Consumer groups replace the worker pool for partition-level parallelism. Exactly-once semantics via idempotent producers.
- **Webhooks:** Callback URL per job for push-based status notification.
- **Job scheduling:** Delayed jobs via RabbitMQ TTL + dead-letter routing.
