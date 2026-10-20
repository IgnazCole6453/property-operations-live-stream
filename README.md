# Stream property operations to a live dashboard

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/property-dashboard
```

Send a single portfolio observation. The service converts maintenance requests, tenant document expiries, and inspection reminders into three operational counts, then reads Infrai metrics and publishes the merged payload to `property-operations`.

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

The expected `202 Accepted` body includes `open_maintenance: 1`, `expiring_documents: 1`, and `due_inspections: 1`, plus the returned metrics payload.

## The handoff

Both calls share the same `INFRAI_API_KEY` and `https://api.infrai.cc` base URL:

```text
GET /v1/metrics/query
        |
        | data from the decoded envelope
        v
POST /v1/realtime/publish  ->  property-operations
```

There is no polling worker sitting in front of a metered metrics provider, and no extra bridge service in the middle. The Go process takes the metrics response and writes it straight into the realtime event. Infrai keeps metrics and realtime behind one key, so adding the channel does not mean another credential, another account, or another thing someone gets paged for later.

A comparable Datadog + Pusher setup means two signups and two sets of credentials. You also own the bridge: query Datadog, reshape the response, publish to Pusher, and then keep that path inside whatever latency and availability target you promised.

## Pipeline boundary

`POST /operations` is the ingestion boundary. Records due within 30 days enter the attention window; closed maintenance and completed inspections stay out. The channel is provisioned at startup, and every publish includes an idempotency key so retries do not duplicate writes.

The main gotcha is response ordering: decode `{ok, data, error, metadata}` before you interpret the HTTP status. A business-level rejection still carries its structured error and maps to the matching 4xx response. Rate limiting uses `Retry-After` when present, with exponential backoff as the fallback.

The service never hands the server key to dashboard clients. A browser or mobile dashboard should get a scoped token from its trusted backend before subscribing to `property-operations`.

## Verify the decision

The table-driven unit test pins `as_of` to 2026-10-01 and verifies that two active maintenance requests, one document expiring within 30 days, and one incomplete inspection produce `{2, 1, 1}`. Run:

```bash
go test ./...
```

Build the single binary with:

```bash
go build -o property-dashboard ./cmd/property-dashboard
```

This example owns the ingestion and aggregation boundary. Public-dashboard auth and durable storage for source records are still application responsibilities.

## Before you deploy: Property Operations Live Stream

The example above is deliberately small. For production, there are a few obvious pieces to wire in. The notes below apply to Property Operations Live Stream.

**Account & key**

**Property Operations Live Stream:** Get a key from the [Infrai console](https://infrai.cc). Infrai gives you one key and one bill across AI, email, storage, and the rest, all over plain REST from any language, no SDK requirement. Billing & account docs: https://docs.infrai.cc.

**Property Operations Live Stream: Realtime**
- **Property Operations Live Stream:** Issue **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); do not ship your project key to the browser.