# Forge Mini

Mini `ArsenalOS/Forge`-style operations recovery system in Go.

This project is intentionally centered on:

- long-running unit workflows
- retries, holds, and rework
- append-only signals and decisions
- fast operational projections
- AI-assisted triage on top of structured history

It is not a CRUD demo.

## Current Slice

The current implementation supports:

- unit admission
- assembly entry and completion
- test start
- flaky test failure with automatic retry
- repeated failure leading to hold and escalation
- quality disposition to route a unit into rework
- rework start and completion
- unit timeline inspection
- incident and station dashboard reads
- deterministic triage summaries
- built-in demo scenario replay

## Run

From the project root:

```bash
go run ./cmd/server
```

The server listens on `:8080`.

## Run Tests

If your environment blocks the default Go cache path, use sandbox-safe paths:

```bash
mkdir -p /tmp/forge-mini-gocache /tmp/forge-mini-gotmp
GOCACHE=/tmp/forge-mini-gocache GOTMPDIR=/tmp/forge-mini-gotmp go test ./...
```

## Demo Scenarios

List built-in scenarios:

```bash
curl http://localhost:8080/api/demo/scenarios
```

Run the `flaky_retry_success` scenario:

```bash
curl -X POST "http://localhost:8080/api/demo/scenarios/flaky_retry_success?unit_id=demo-unit-1"
```

Run the `repeated_failure_rework` scenario:

```bash
curl -X POST "http://localhost:8080/api/demo/scenarios/repeated_failure_rework?unit_id=demo-unit-2"
```

Inspect the workflow after a scenario:

```bash
curl http://localhost:8080/api/units/demo-unit-2
```

Inspect the unit timeline:

```bash
curl http://localhost:8080/api/units/demo-unit-2/timeline
```

Inspect incidents:

```bash
curl http://localhost:8080/api/dashboard/incidents
```

Inspect station health:

```bash
curl http://localhost:8080/api/dashboard/stations
```

Inspect timers:

```bash
curl http://localhost:8080/api/timers
```

Get triage output for a unit:

```bash
curl http://localhost:8080/api/triage/units/demo-unit-2
```

## Manual Curl Flow

If you want to drive the workflow by hand instead of using the demo endpoint:

Admit a unit:

```bash
curl -X POST http://localhost:8080/api/signals \
  -H "Content-Type: application/json" \
  -d '{
    "id": "sig-1",
    "type": "unit.admitted",
    "unit_id": "manual-unit-1",
    "source": "planner",
    "occurred_at": "2026-04-25T12:00:00Z"
  }'
```

Enter assembly:

```bash
curl -X POST http://localhost:8080/api/signals \
  -H "Content-Type: application/json" \
  -d '{
    "id": "sig-2",
    "type": "station.entered",
    "unit_id": "manual-unit-1",
    "source": "assembly-station-1",
    "station_id": "assembly",
    "occurred_at": "2026-04-25T12:00:01Z"
  }'
```

Complete assembly:

```bash
curl -X POST http://localhost:8080/api/signals \
  -H "Content-Type: application/json" \
  -d '{
    "id": "sig-3",
    "type": "assembly.completed",
    "unit_id": "manual-unit-1",
    "source": "assembly-station-1",
    "station_id": "assembly",
    "occurred_at": "2026-04-25T12:00:02Z"
  }'
```

Start test:

```bash
curl -X POST http://localhost:8080/api/signals \
  -H "Content-Type: application/json" \
  -d '{
    "id": "sig-4",
    "type": "test.started",
    "unit_id": "manual-unit-1",
    "source": "test-bench-2",
    "station_id": "test",
    "occurred_at": "2026-04-25T12:00:03Z"
  }'
```

Fail test with a retryable failure:

```bash
curl -X POST http://localhost:8080/api/signals \
  -H "Content-Type: application/json" \
  -d '{
    "id": "sig-5",
    "type": "test.failed",
    "unit_id": "manual-unit-1",
    "source": "test-bench-2",
    "station_id": "test",
    "occurred_at": "2026-04-25T12:00:04Z",
    "payload": {
      "failure_code": "fixture_comm_error",
      "failure_class": "flaky_infra",
      "message": "lost connection to fixture controller"
    }
  }'
```

Then inspect timers and fire the retry timer manually:

```bash
curl http://localhost:8080/api/timers
```

```bash
curl -X POST http://localhost:8080/api/timers \
  -H "Content-Type: application/json" \
  -d '{
    "key": "manual-unit-1:retry_backoff_timer:test",
    "workflow_id": "wf-manual-unit-1",
    "unit_id": "manual-unit-1",
    "timer_type": "retry_backoff_timer",
    "station_id": "test",
    "fire_at": "2026-04-25T12:00:07Z"
  }'
```

Then pass the test:

```bash
curl -X POST http://localhost:8080/api/signals \
  -H "Content-Type: application/json" \
  -d '{
    "id": "sig-6",
    "type": "test.passed",
    "unit_id": "manual-unit-1",
    "source": "test-bench-2",
    "station_id": "test",
    "occurred_at": "2026-04-25T12:00:08Z"
  }'
```

## Key Endpoints

- `POST /api/signals`
- `POST /api/timers`
- `GET /api/timers`
- `GET /api/units/{unitID}`
- `GET /api/units/{unitID}/timeline`
- `GET /api/dashboard/incidents`
- `GET /api/dashboard/stations`
- `GET /api/dashboard/units?state=on_hold`
- `GET /api/triage/units/{unitID}`
- `GET /api/demo/scenarios`
- `POST /api/demo/scenarios/{name}?unit_id=...`

## Current Gaps

Things that are intentionally still thin:

- timer execution is manual
- the AI layer is deterministic, not model-backed
- no frontend yet
- no persistent database yet
- no broader station-level incident model yet

That is okay for this phase. The current slice is meant to prove the orchestration model.
