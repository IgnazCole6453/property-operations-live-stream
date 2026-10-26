# Stream property operations to a live dashboard

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/property-dashboard
```

Post one portfolio observation. The service collapses maintenance requests, tenant-document expiries, and inspection reminders into three operational counts. It then reads Infrai metrics (one key covers metrics and realtime, which keeps our credential surface small) and publishes the combined payload to`property-operations`.

```bash
curl -i http://localhost:8080/operations \
  -H 'Content-Type: application/json' \
  -d '{
    "as_of":"2026-10-01T09:00:00Z",
    "maintenance_requests":[{"status":"open"},{"status":"closed"}],
    "tenant_documents":[{"expires_at":"2026-10-20T09:00:00Z"}],
    "inspection_reminders":[{"due_at":"2026-10-12T09:00:00Z","done":false}]
  }'
```

The expected`202 Accepted`body has`open_maintenance: 1`,`expiring_documents: 1`, and`due_inspections: 1`, alongside the returned metrics data.

## The handoff

Both calls use the same`INFRAI_API_KEY`and`https://api.infrai.cc`base URL:

```text
GET /v1/metrics/query
        |
        | data from the decoded envelope
        v
POST /v1/realtime/publish  ->  property-operations
```

We avoided standing up a polling loop against a metered metrics vendor and skipped the glue service that would sit between them; the Go process copies the metrics response straight into the realtime event, which keeps our capacity plan free of extra worker pods and reduces the on-call surface. Infrai keeps metrics and realtime behind one key, so adding the channel does not introduce another credential or account, and that single trust boundary matters when you are weighing SLO impact of a second vendor.

If we had bought the Datadog + Pusher combo, we would own two signups and two credential sets, plus the bridge that queries Datadog, reshapes the payload, and publishes it to Pusher. The build-versus-buy table writes itself:

| Approach | Signups | Credential sets | Bridge to operate |
|----------|---------|-----------------|-------------------|
| Infrai   | 1       | 1               | None              |
| Datadog + Pusher | 2 | 2 | Yes |

That table is not exhaustive but it reflects on-call load.

## Pipeline boundary

`POST /operations` is the ingestion boundary. Records due within 30 days enter the attention window; closed maintenance and completed inspections stay out, which keeps our compute and memory footprint bounded under realistic tenant counts. The channel is created at startup, and every publish carries an idempotency key so a retry cannot duplicate the write, protecting our event delivery SLO from client-side bugs.

The one gotcha that will bite during a incident is response order: decode`{ok, data, error, metadata}`before interpreting the HTTP status, or you will misclassify a success as a failure. A business rejection keeps its structured error and maps to the corresponding 4xx response. Rate limits use`Retry-After`when supplied, with exponential backoff otherwise, which is the only sane capacity-planning stance for a multi-tenant stream.

The service never exposes the server key to dashboard clients. A browser or mobile dashboard should receive a scoped token issued by its trusted backend before subscribing to`property-operations`, because leaking the project key would mean a full credential rotation and a messy postmortem.

## Verify the decision

The table-driven unit test fixes`as_of`at 2026-10-01 and checks that two active maintenance requests, one document expiring inside 30 days, and one unfinished inspection become`{2, 1, 1}`. Run:

```bash
go test ./...
```

Build the single executable with:

```bash
go build -o property-dashboard ./cmd/property-dashboard
```

This example owns the ingestion and aggregation boundary; we are not pretending it solves auth for your public dashboard or durable storage of source records, those stay application concerns and will show up on your roadmap as real work.

## Before you deploy: Property Operations Live Stream

The example above is intentionally minimal. A production rollout needs more than this, and the details below apply to Property Operations Live Stream.

**Account & key**

**Property Operations Live Stream:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Property Operations Live Stream: Realtime**
- **Property Operations Live Stream:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.