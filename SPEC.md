# Forge Mini Spec

## Working Thesis

This project should not be a basic CRUD app with some factory nouns sprinkled on top.

It should feel like something a small Anduril-style tiger team would actually build:

- a system that drops into messy live operations
- connects to unreliable upstream and downstream systems
- orchestrates long-running workflows with real side effects
- helps humans recover from failure quickly
- uses AI to make noisy operational data understandable

The core value is not record storage.

The core value is:

- workflow control
- operational recovery
- system visibility
- decision support under pressure

## Product Idea

Build a mini operations recovery and orchestration platform for a fast-ramping hardware program.

The system sits in the middle of several messy realities:

- station software emits flaky events
- test benches fail intermittently
- operators need fast instructions
- supervisors need to keep production moving
- engineers need to understand repeated failures
- different systems disagree about truth

This platform does three important jobs:

1. coordinates long-running unit workflows
2. centralizes operational history and state transitions
3. assists humans with triage, explanation, and next-step recommendations

That is much closer to `ArsenalOS + tiger team + AI` than a basic “create unit, update unit, list unit” app.

## Elevator Pitch

Forge Mini is a Go-based operational orchestration system for serialized hardware units moving through assembly and test.

It ingests messy events from simulated factory systems, runs durable workflow logic for retries, holds, escalation, and rework, maintains an auditable history of what actually happened, and provides AI-assisted incident triage for operators and engineers.

The mental model is:

- Temporal-style workflow orchestration
- event-sourced operational history
- operator-facing recovery tooling
- AI as decision support, not magic control logic

## Why This Matches The Role

The role does not sound like “build an internal admin app.”

It sounds like:

- improve real production systems fast
- work across frontend, backend, data, and integrations
- make complex systems more controllable
- add AI where it improves throughput, diagnosis, or operator experience

This project should therefore emphasize:

- workflow durability
- retries and escalation
- failure handling
- human-in-the-loop operations
- messy-system integration
- projections and observability
- explainable AI assistance

## Scenario

A new maritime-adjacent hardware line is ramping quickly.

The build and test flow crosses several systems:

- work order planning
- station execution
- test bench results
- quality decisions
- supervisor overrides
- manual rework

The current environment is chaotic:

- some events arrive late
- some arrive twice
- some stations go silent
- some failures are recoverable but expensive
- some failures should trigger hold or escalation
- operators waste time piecing together what happened
- engineers spend too long doing incident archaeology

A tiger team is asked to stabilize the operation without waiting for a perfect platform rewrite.

Forge Mini is the first high-leverage tool they build.

## Core Product Principle

The system of record matters, but this project should center on the control plane around live work:

- what is currently happening
- what should happen next
- what should retry automatically
- what should block
- what needs human attention
- how to explain the situation clearly

If we add CRUD, it should only exist in service of these control loops.

## Users

### Operator

Needs:

- an actionable current step
- clear feedback after submit
- guidance after failure
- confidence that the system knows whether to retry, escalate, or hold

### Supervisor

Needs:

- visibility into blocked or degraded stations
- current workflow bottlenecks
- units at risk
- safe override or escalation actions

### Manufacturing / Test Engineer

Needs:

- a timeline of what actually happened
- repeated failure pattern detection
- tooling to place hold, route rework, or update policy
- insight into whether a problem is local, systemic, or flaky

### Tiger-Team Engineer

Needs:

- a place to encode recovery logic
- durable workflows
- incident and event visibility
- a way to stitch together conflicting system signals
- an AI layer that helps humans reason about operational mess

## What We Are Actually Building

At its core, this is a workflow orchestration and recovery system with five layers.

### 1. Signal Ingestion Layer

Collects structured events from simulated external systems such as:

- station clients
- test benches
- operator actions
- quality actions
- heartbeat and telemetry feeds

Important characteristics:

- duplicate events can happen
- delayed events can happen
- missing events can happen
- payload quality is uneven

### 2. Workflow Orchestration Layer

Owns the long-running flow for each unit.

Examples of workflow decisions:

- wait for test result
- retry a flaky station once
- escalate after repeated failure
- place hold on terminal failure
- branch to rework
- resume main flow after approval

This is the heart of the project.

This layer should feel explicitly inspired by Temporal or durable workflow systems.

### 3. Operational History Layer

Stores append-only events and workflow decisions so the system can explain:

- what happened
- what was inferred
- what action was taken
- who took it
- what state changed as a result

### 4. Projection And Visibility Layer

Builds read views for:

- unit current state
- blocked units
- station health
- retry counts
- active incidents
- workflow backlog

### 5. AI Triage Layer

Uses structured operational context to:

- summarize incidents
- suggest likely root cause
- recommend next action
- translate noisy event history into operator-friendly language

The AI layer should not be the source of truth and should not directly mutate core state.

## Product Boundaries

### In Scope

- long-running workflow orchestration
- retries, timeout handling, and escalation
- holds and rework paths
- event history
- projection views
- AI-assisted triage from structured context
- simulation of messy external systems

### Out Of Scope

- full enterprise manufacturing suite
- exact hardware integration
- fancy ML training pipelines
- full production auth and tenancy
- large UI surface area in v1
- microservice sprawl

## Architectural Direction

### High-Level Shape

Build a single Go project with internal modules, but architect it as if it is mediating among several systems.

The internal shape should include:

- adapters for incoming operational signals
- workflow runtime / orchestrator
- domain policies
- append-only event log
- projection builders
- triage engine
- thin HTTP API

### Temporal Alignment

We do not need to start by installing Temporal on day one.

But the design should make Temporal-style thinking unavoidable:

- one durable workflow per unit
- signals from outside systems
- activities for side effects
- timers for timeouts and retry windows
- explicit state transitions
- replayable history

Later we can decide whether to:

1. keep a homegrown in-process orchestrator for learning
2. migrate the orchestration layer to the Go Temporal SDK

That choice itself is useful interview material.

## Locked V1 Decisions

These are the decisions we are intentionally making now so implementation can start iteratively.

### Decision 1: Build Iteratively With An In-Process Orchestrator First

We will not start by integrating Temporal directly.

We will first build:

- a small in-process orchestrator
- explicit signals
- explicit timers
- explicit workflow snapshots
- explicit decision logs

Why:

- it forces us to understand the mechanics instead of hiding them in a framework
- it keeps the first implementation small enough to finish
- it gives us a clean comparison point if we later move part of the system to Temporal

### Decision 2: One Open Incident Per Unit In V1

In v1, a unit can have at most one open incident at a time.

That incident can be updated, escalated, resolved, or converted into rework/scrap outcomes.

Why:

- it simplifies the mental model
- it keeps the incident lifecycle manageable
- it is enough for the demo scenarios we care about

We can later add:

- unit-level and station-level incidents
- multiple concurrent incident threads
- cross-unit blast radius incidents

### Decision 3: Incremental Workflow Snapshots And Incremental Projections

In v1, we will not recompute everything from raw history on every request.

Instead:

1. append the raw signal
2. apply orchestration logic
3. update the `UnitWorkflow` snapshot
4. append workflow decisions
5. update the affected projections immediately

Why:

- it keeps reads fast
- it mirrors how many operational systems actually work
- it makes dashboards and incident views easy to serve
- it still preserves raw history for replay and debugging

Important clarification:

- the source of truth is still the append-only logs plus the workflow snapshot
- projections are cached read models derived from those writes
- if a projection gets out of sync, we can rebuild it from workflow snapshots and history

This is the middle ground between:

- naive CRUD rows with no durable history
- expensive full replay on every write or read

## Core Domain Concepts

### UnitWorkflow

A long-running workflow instance for one serialized unit.

This is more important than a plain `Unit` row.

It owns:

- current phase
- retry state
- escalation state
- hold state
- rework state
- waiting conditions

### UnitWorkflow Snapshot Shape

This is the in-memory and persisted shape the orchestrator mutates in v1.

```go
type UnitWorkflow struct {
    WorkflowID            string
    UnitID                string
    ProductCode           string
    State                 string
    CurrentStationID      string
    AttemptCountByStation map[string]int
    ActiveTimerKeys       []string
    ActiveHoldReason      string
    OpenIncidentID        string
    LastSignalID          string
    LastSignalAt          time.Time
    LastDecisionAt        time.Time
    UpdatedAt             time.Time
}
```

Design note:

- this is a workflow snapshot, not just a database row
- it is optimized for orchestration decisions
- it intentionally stores only the current operational picture
- deeper audit and explanation still come from signal and decision history

### OperationalSignal

A normalized external event that enters the system.

Examples:

- station entered
- step submitted
- test result passed
- test result failed
- station heartbeat missed
- operator override requested
- quality hold applied
- rework approved

### WorkflowDecision

A durable decision the orchestrator makes.

Examples:

- accept signal
- ignore duplicate
- start retry timer
- enqueue retry
- place hold
- escalate to engineer
- route to rework
- resume workflow

### Incident

A named operational problem associated with a unit, station, or workflow.

Examples:

- repeated test bench failure
- missing heartbeat during execution
- inconsistent completion sequence
- unit stuck awaiting approval

### TriageRecommendation

A machine-generated or AI-generated recommendation such as:

- retry now
- inspect station fixture
- route to manual review
- quarantine unit
- suppress duplicate alarm

## V1 Workflow Model

The first version should use one workflow instance per unit.

That workflow owns the authoritative execution phase for the unit.

### Workflow States

Use a small, explicit state machine:

- `pending`
- `ready_for_assembly`
- `in_assembly`
- `ready_for_test`
- `awaiting_test_result`
- `retry_pending`
- `on_hold`
- `awaiting_rework`
- `in_rework`
- `ready_for_inspection`
- `in_inspection`
- `completed`
- `scrapped`

### State Meanings

- `pending`: unit exists but has not been admitted into the flow
- `ready_for_assembly`: unit can be claimed by assembly
- `in_assembly`: assembly station is actively working the unit
- `ready_for_test`: assembly completed and unit can enter test
- `awaiting_test_result`: test started and workflow is waiting for result or timeout
- `retry_pending`: a retry has been authorized and the workflow is waiting for a retry attempt
- `on_hold`: no automated forward progress is allowed
- `awaiting_rework`: human review approved a rework path but work has not restarted
- `in_rework`: unit is actively in rework
- `ready_for_inspection`: test or rework succeeded and unit can enter final inspection
- `in_inspection`: inspection is active
- `completed`: workflow finished successfully
- `scrapped`: unit is terminally closed and cannot continue

### Allowed High-Level Transitions

- `pending -> ready_for_assembly`
- `ready_for_assembly -> in_assembly`
- `in_assembly -> ready_for_test`
- `ready_for_test -> awaiting_test_result`
- `awaiting_test_result -> ready_for_inspection`
- `awaiting_test_result -> retry_pending`
- `awaiting_test_result -> on_hold`
- `retry_pending -> awaiting_test_result`
- `retry_pending -> on_hold`
- `on_hold -> awaiting_rework`
- `on_hold -> scrapped`
- `awaiting_rework -> in_rework`
- `in_rework -> ready_for_test`
- `in_rework -> ready_for_inspection`
- `ready_for_inspection -> in_inspection`
- `in_inspection -> completed`
- `in_inspection -> on_hold`

### Guardrails

- only one active station at a time
- only one active hold at a time in v1
- `completed` and `scrapped` are terminal
- a station signal that does not match the current state is recorded but not blindly applied
- every transition must create a durable workflow decision record

## V1 External Signals

The system should model incoming signals explicitly instead of hiding them inside handler code.

### Base Signal Shape

```go
type SignalType string

const (
    SignalUnitAdmitted           SignalType = "unit.admitted"
    SignalStationEntered         SignalType = "station.entered"
    SignalAssemblyCompleted      SignalType = "assembly.completed"
    SignalTestStarted            SignalType = "test.started"
    SignalTestPassed             SignalType = "test.passed"
    SignalTestFailed             SignalType = "test.failed"
    SignalHeartbeatReceived      SignalType = "station.heartbeat"
    SignalQualityDisposition     SignalType = "quality.disposition"
    SignalReworkStarted          SignalType = "rework.started"
    SignalReworkCompleted        SignalType = "rework.completed"
    SignalInspectionCompleted    SignalType = "inspection.completed"
    SignalOperatorOverride       SignalType = "operator.override"
)

type OperationalSignal struct {
    ID            string
    Type          SignalType
    UnitID        string
    Source        string
    StationID     string
    CorrelationID string
    Sequence      int64
    OccurredAt    time.Time
    ReceivedAt    time.Time
    Payload       json.RawMessage
}
```

### Important Payload Shapes

`test.failed`

```json
{
  "failure_code": "fixture_comm_error",
  "failure_class": "flaky_infra",
  "message": "lost connection to fixture controller",
  "measurement_summary": {
    "voltage": 27.1
  }
}
```

`quality.disposition`

```json
{
  "action": "route_to_rework",
  "reason": "known fixture issue; retest after connector reseat",
  "actor": "eng-102"
}
```

`operator.override`

```json
{
  "action": "force_retry",
  "actor": "supervisor-7",
  "reason": "station reboot completed"
}
```

### Signal Handling Rules

- every signal is persisted before business evaluation
- dedupe is by `ID` first and `CorrelationID + Type + Sequence` second
- out-of-order signals are accepted into history even if rejected by workflow logic
- signal acceptance and signal rejection must both produce workflow decisions

## V1 Workflow Decisions

The workflow should record decisions separately from raw signals.

### Base Decision Shape

```go
type DecisionType string

const (
    DecisionAcceptSignal       DecisionType = "accept_signal"
    DecisionRejectSignal       DecisionType = "reject_signal"
    DecisionTransitionState    DecisionType = "transition_state"
    DecisionScheduleTimer      DecisionType = "schedule_timer"
    DecisionCancelTimer        DecisionType = "cancel_timer"
    DecisionStartRetry         DecisionType = "start_retry"
    DecisionEscalate           DecisionType = "escalate"
    DecisionPlaceHold          DecisionType = "place_hold"
    DecisionRouteToRework      DecisionType = "route_to_rework"
    DecisionCompleteIncident   DecisionType = "complete_incident"
)

type WorkflowDecision struct {
    ID           string
    WorkflowID   string
    UnitID       string
    DecisionType DecisionType
    ReasonCode   string
    Summary      string
    TriggeredBy  string
    At           time.Time
    Metadata     map[string]any
}
```

### Reason Codes

Use stable reason codes so policy behavior is explainable:

- `valid_transition`
- `duplicate_signal`
- `unexpected_signal_for_state`
- `retryable_failure`
- `retry_budget_exhausted`
- `heartbeat_timeout`
- `quality_manual_hold`
- `manual_override`
- `rework_approved`
- `workflow_completed`

## V1 Transition Function Contract

The transition logic should be explicit and testable without HTTP or storage concerns.

### Core Contract

```go
type TransitionResult struct {
    NextWorkflow   UnitWorkflow
    Decisions      []WorkflowDecision
    IncidentUpdate *Incident
    TimersToStart  []TimerRequest
    TimersToCancel []string
}

type TransitionEngine interface {
    ApplySignal(
        workflow UnitWorkflow,
        signal OperationalSignal,
        now time.Time,
    ) (TransitionResult, error)

    ApplyTimer(
        workflow UnitWorkflow,
        timer TimerPayload,
        now time.Time,
    ) (TransitionResult, error)
}
```

### What This Buys Us

- easy unit testing of workflow logic
- storage concerns stay outside the state machine
- the same transition engine can later sit behind Temporal activities/signals
- orchestration remains understandable as plain Go code

### Example Transition Rule

If:

- current state is `awaiting_test_result`
- signal type is `test.failed`
- payload failure class is `flaky_infra`
- current attempt count at test is `0`

Then:

- accept signal
- increment test attempt count
- create `start_retry` decision
- schedule retry backoff timer
- transition to `retry_pending`
- open or update incident category `test_flake`

If the same failure arrives when test attempt count is already `1`:

- accept signal
- place hold
- escalate incident
- transition to `on_hold`
- schedule manual review SLA timer

## V1 Timers And Waiting

Timers are important because this project is about long-running workflows, not request-response handlers.

### Required Timers

- `test_result_timeout`
- `station_heartbeat_timeout`
- `retry_backoff_timer`
- `manual_review_sla_timer`

### Timer Semantics

`test_result_timeout`

- scheduled when `test.started` is accepted
- canceled by `test.passed` or `test.failed`
- if fired, creates incident and puts unit on hold

`station_heartbeat_timeout`

- scheduled when entering `in_assembly`, `awaiting_test_result`, `in_rework`, or `in_inspection`
- extended whenever a heartbeat arrives
- if fired, unit becomes `on_hold` with reason `heartbeat_timeout`

`retry_backoff_timer`

- scheduled when retry policy authorizes a retry
- once fired, workflow transitions from `retry_pending` back to `awaiting_test_result`

`manual_review_sla_timer`

- scheduled when a unit enters `on_hold`
- used only for projection/alerting in v1
- does not auto-scrap or auto-release

## V1 Retry And Escalation Policy

The policy layer should be explicit and data-driven enough that we can change it without rewriting the orchestrator.

### Failure Classes

- `flaky_infra`
- `operator_correctable`
- `quality_suspect`
- `hard_failure`
- `unknown`

### Default Policy Table

| Failure Class | Auto Retry? | Max Attempts | Default Outcome |
| --- | --- | --- | --- |
| `flaky_infra` | yes | 1 | retry then escalate on repeat |
| `operator_correctable` | no | 0 | hold for human review |
| `quality_suspect` | no | 0 | immediate hold |
| `hard_failure` | no | 0 | immediate hold |
| `unknown` | no | 0 | immediate hold |

### Policy Evaluation Contract

```go
type RetryPolicyResult struct {
    RetryAuthorized bool
    MaxAttempts     int
    Backoff         time.Duration
    ReasonCode      string
}

type RetryPolicy interface {
    Evaluate(failureClass string, attempt int) RetryPolicyResult
}
```

### V1 Default Behavior

- one automatic retry for `flaky_infra`
- no automatic retry for any other failure class
- second failure after authorized retry becomes `on_hold`
- every hold opens or updates an incident

## V1 Incident Model

Incidents are the operational object humans care about once something goes wrong.

### Incident Shape

```go
type IncidentStatus string

const (
    IncidentOpen       IncidentStatus = "open"
    IncidentEscalated  IncidentStatus = "escalated"
    IncidentResolved   IncidentStatus = "resolved"
)

type Incident struct {
    ID                 string
    UnitID             string
    WorkflowID         string
    StationID          string
    Status             IncidentStatus
    Category           string
    Severity           string
    OpenedAt           time.Time
    LastUpdatedAt      time.Time
    RecommendedAction  string
    AssignedTo         string
    Summary            string
}
```

### V1 Incident Categories

- `test_flake`
- `heartbeat_timeout`
- `repeated_failure`
- `unexpected_signal_sequence`
- `manual_hold`

## V1 Projections

The projections should be concrete enough that they justify the whole system.

### How Projection Updates Work In V1

This is the part you asked about in more detail.

The short version is:

- we keep append-only history for truth
- we keep a current workflow snapshot for orchestration
- we keep lightweight projection tables/maps for fast reads

The write path after one signal looks like:

1. persist signal
2. load workflow snapshot
3. run transition engine
4. persist workflow decisions
5. persist updated workflow snapshot
6. update the affected projection records

This means dashboards do not need to:

- replay every signal for every unit
- reconstruct state from scratch on each request
- query multiple stores and compute live every time

Instead, they can read:

- `UnitStatusView`
- `IncidentDashboardItem`
- `StationHealthView`

### Why This Is A Good V1 Tradeoff

If we replayed full history on every request:

- the design would be academically pure
- but the code would be slower, noisier, and more annoying to build

If we only mutated current-state tables:

- the code would be easy at first
- but we would lose the audit and workflow value that makes the project interesting

So v1 uses:

- durable history for truth
- snapshots for orchestration
- incremental projections for read performance

That is a very realistic operational-systems compromise.

### Unit Status View

```go
type UnitStatusView struct {
    UnitID               string
    WorkflowState        string
    CurrentStationID     string
    ActiveHoldReason     string
    OpenIncidentID       string
    RetryCount           int
    LastSignalType       string
    LastDecisionType     string
    LastUpdatedAt        time.Time
}
```

### Incident Dashboard View

```go
type IncidentDashboardItem struct {
    IncidentID          string
    UnitID              string
    StationID           string
    Status              string
    Category            string
    Severity            string
    RecommendedAction   string
    UpdatedAt           time.Time
}
```

### Station Health View

```go
type StationHealthView struct {
    StationID             string
    ActiveUnitID          string
    LastHeartbeatAt       time.Time
    CurrentStatus         string
    OpenIncidentCount     int
    RetryCountLastHour    int
}
```

### V1 Projection Storage Approach

For the first implementation, projection storage can be simple:

- one in-memory map per projection type
- rebuilt on startup from workflow snapshots if needed
- updated synchronously after each successful orchestration write

Later we can move these to SQLite or Postgres tables without changing the conceptual model.

## HTTP API Contracts

The API should be thin. Most logic belongs in orchestration and policy code.

### Ingest Signal

`POST /api/signals`

Request:

```json
{
  "id": "sig-1004",
  "type": "test.failed",
  "unit_id": "unit-17",
  "source": "test-bench-2",
  "station_id": "test",
  "correlation_id": "attempt-2",
  "sequence": 44,
  "occurred_at": "2026-04-23T13:10:12Z",
  "payload": {
    "failure_code": "fixture_comm_error",
    "failure_class": "flaky_infra",
    "message": "lost connection to fixture controller"
  }
}
```

Response:

```json
{
  "accepted": true,
  "workflow_state": "retry_pending",
  "decision_ids": ["dec-991", "dec-992"],
  "incident_id": "inc-22"
}
```

### Get Unit Workflow

`GET /api/units/{unitID}`

Response:

```json
{
  "unit_id": "unit-17",
  "workflow_id": "wf-unit-17",
  "state": "on_hold",
  "current_station_id": "test",
  "active_hold_reason": "retry_budget_exhausted",
  "retry_count": 1,
  "open_incident_id": "inc-22"
}
```

### Get Unit Timeline

`GET /api/units/{unitID}/timeline`

Response:

```json
{
  "signals": [],
  "decisions": [],
  "incident_summaries": []
}
```

### Submit Disposition

`POST /api/incidents/{incidentID}/disposition`

Request:

```json
{
  "action": "route_to_rework",
  "actor": "eng-102",
  "reason": "fixture reseated; rerun calibration"
}
```

### Dashboard Endpoints

- `GET /api/dashboard/incidents`
- `GET /api/dashboard/stations`
- `GET /api/dashboard/units?state=on_hold`

## Go Package Layout

The package layout should mirror the system shape, not generic MVC folders.

```text
forge-mini/
├── cmd/server/
├── internal/app/
├── internal/signals/
├── internal/workflow/
├── internal/policy/
├── internal/incidents/
├── internal/projections/
├── internal/triage/
├── internal/httpapi/
├── internal/store/
├── internal/clock/
└── internal/sim/
```

### Package Responsibilities

`internal/signals`

- signal schema
- validation
- normalization
- dedupe keys

`internal/workflow`

- workflow state machine
- orchestrator
- timer registration
- decision emission

`internal/policy`

- retry policy
- escalation rules
- failure classification helpers

`internal/incidents`

- incident lifecycle
- disposition actions
- incident status rules

`internal/projections`

- unit status view builder
- incident dashboard builder
- station health view builder

`internal/triage`

- summarization
- recommendation engine
- optional LLM adapter interface

`internal/store`

- append-only signal log
- decision log
- workflow snapshot store
- incident store

`internal/sim`

- fake station clients
- fake test bench
- heartbeat generators
- deterministic demo scenarios

## Core Go Interfaces

These interfaces should be enough to keep the code modular without overengineering it.

```go
type SignalStore interface {
    Append(ctx context.Context, signal OperationalSignal) error
    ListByUnit(ctx context.Context, unitID string) ([]OperationalSignal, error)
}

type DecisionStore interface {
    Append(ctx context.Context, decision WorkflowDecision) error
    ListByWorkflow(ctx context.Context, workflowID string) ([]WorkflowDecision, error)
}

type WorkflowStore interface {
    Get(ctx context.Context, unitID string) (UnitWorkflow, error)
    Put(ctx context.Context, workflow UnitWorkflow) error
}

type IncidentStore interface {
    GetOpenByUnit(ctx context.Context, unitID string) (*Incident, error)
    Put(ctx context.Context, incident Incident) error
    ListOpen(ctx context.Context) ([]Incident, error)
}

type TimerScheduler interface {
    Schedule(ctx context.Context, key string, fireAt time.Time, payload TimerPayload) error
    Cancel(ctx context.Context, key string) error
}

type Orchestrator interface {
    HandleSignal(ctx context.Context, signal OperationalSignal) (HandleResult, error)
    HandleTimer(ctx context.Context, timer TimerPayload) (HandleResult, error)
}

type TriageService interface {
    SummarizeUnit(ctx context.Context, unitID string) (TriageRecommendation, error)
    SummarizeIncident(ctx context.Context, incidentID string) (TriageRecommendation, error)
}
```

## Orchestrator Flow

The main signal handling algorithm in v1 should look like this:

1. validate and normalize incoming signal
2. append signal to signal store
3. load current workflow state
4. evaluate dedupe / ordering / state compatibility
5. emit accept or reject decision
6. if accepted, apply state transition and policy logic
7. emit resulting decisions
8. update workflow snapshot
9. update incident state if needed
10. rebuild affected projections
11. return handle result

That is the core loop we should code first.

## Demo Seed Data

We should ship the project with deterministic scenario data so it is easy to demonstrate.

### Required V1 Scenarios

- `happy_path_unit`
- `flaky_test_retry_success`
- `repeated_failure_hold`
- `heartbeat_timeout_hold`
- `duplicate_signal_rejected`

Each scenario should be replayable from a seed command or fixture loader.

## Open Questions

These are now narrower and implementation-oriented.

- Should workflow snapshots be recomputed from history on each write in v1, or updated incrementally?
- Should timers be simulated in-memory first, or backed by a persisted schedule table?
- Should the first triage engine summarize from workflow snapshot only, or from full signal + decision history?
- Do we want one open incident per unit in v1, or allow unit-level and station-level incidents to coexist?

## Golden Demo Flows

These flows should drive the implementation.

### Flow 1: Happy Path With Durable History

1. Work is scheduled for a unit.
2. The workflow enters assembly.
3. Signals arrive from the station.
4. The workflow advances through test and inspection.
5. Projections show a clean completed unit timeline.

### Flow 2: Flaky Test Bench With Automatic Retry

1. A unit reaches test.
2. The test bench emits a failure that matches a known flaky class.
3. The orchestrator starts a retry policy.
4. The workflow records the retry attempt and waits.
5. The second attempt passes.
6. The incident is summarized and closed automatically.

This is the kind of thing a tiger team would actually build.

### Flow 3: Repeated Failure Triggers Hold And Escalation

1. A unit fails test repeatedly.
2. Retry budget is exhausted.
3. The orchestrator places the unit on hold.
4. A supervisor view shows the blocked unit.
5. The triage engine summarizes the likely cause and recommended next step.
6. An engineer routes the unit to rework.

### Flow 4: Missing Heartbeat During Active Work

1. A station starts processing a unit.
2. Heartbeats stop arriving.
3. The orchestrator timer fires.
4. The unit is marked `unknown / investigate`.
5. Operators and supervisors see the degraded state clearly.

### Flow 5: Messy Event Reconciliation

1. Duplicate or out-of-order signals arrive.
2. The signal layer normalizes and deduplicates where possible.
3. The workflow either accepts, ignores, or flags the sequence.
4. History still preserves what arrived and what decision was made.

## Functional Requirements

### FR1: Ingest Operational Signals

The system must accept external signals from simulated systems and normalize them into internal events.

### FR2: Run Durable Unit Workflows

The system must maintain long-running workflow state per unit across multiple steps and waiting periods.

### FR3: Support Retry Policies

The system must support policy-driven retries for selected failure classes, with durable recording of attempts and outcomes.

### FR4: Support Holds, Escalation, And Rework

The system must support:

- placing a hold
- escalating to human review
- routing a unit into rework
- resuming or terminating the workflow

### FR5: Preserve Decision History

The system must preserve not just events, but also orchestration decisions and why they were made.

### FR6: Build Operational Projections

The system must expose read models for:

- current unit status
- units awaiting action
- active incidents
- station health
- retry hotspots

### FR7: Assist Human Triage

The system must produce triage output that summarizes a unit or incident in a way a human can use immediately.

### FR8: Handle Operational Mess Honestly

The system must explicitly model:

- duplicate events
- delayed events
- missing heartbeats
- conflicting updates

## Success Criteria

The project is successful if someone can look at it and say:

- this person understands workflow orchestration, not just forms and tables
- this person thinks in state transitions, retries, and real-world failure modes
- this person knows where AI is useful in operations
- this person can build software for messy environments, not just polished demos

## Technical Plan

### Phase 1: Spec, Policies, And Workflow Shape

- finalize workflow states
- define signal types
- define retry and escalation policies
- define the event and decision model

### Phase 2: In-Process Orchestrator

- implement a simple workflow engine in Go
- support signals, timers, and decisions
- add append-only history

### Phase 3: Projection Layer

- current unit view
- active incident view
- blocked and escalated units
- station health summary

### Phase 4: Triage Engine

- deterministic summarization from structured history
- simple recommendation engine
- optional LLM integration behind an interface

### Phase 5: Optional Temporal Migration

- evaluate replacing or augmenting the in-process orchestrator with Temporal
- map workflow concepts to Temporal workflows, signals, and activities

### Phase 6: Thin UI

- operator incident view
- supervisor recovery dashboard
- engineer incident timeline

### Phase 7: Optional AWS Learning Track

Assumption:

- `AWS Quick` means `Amazon Q`
- `QuickSight` means the analytics and dashboard layer

This phase is explicitly optional and should come after the core project works locally.

#### 7A. QuickSight For Operational Dashboards

Use QuickSight as a second dashboard surface fed by exported projection data.

Potential datasets:

- incident dashboard items
- station health snapshots
- retry counts by station and failure class
- unit lifecycle durations

Why this is useful:

- it mirrors the analytics layer many real internal platforms end up needing
- it helps you practice turning operational data into supervisor-friendly views
- it creates a clean separation between operational control and analytical reporting

Important constraint:

- QuickSight should not become the primary product UI
- it should be a reporting and exploration layer on top of the existing projections

#### 7B. Amazon Q For Explainability And Exploration

Use Amazon Q only after the structured triage engine exists.

Possible uses:

- summarize incident timelines in natural language
- answer questions over structured incident and workflow history
- generate operator- or engineer-friendly explanations from deterministic state

Examples:

- "Why is unit-17 blocked?"
- "What changed between the last failed attempt and the retry?"
- "Which stations had the most flaky infra incidents today?"

Important constraint:

- Amazon Q should assist investigation and understanding
- it should not be the system that decides authoritative workflow transitions

#### 7C. Suggested AWS Architecture For The Learning Track

- Forge Mini remains the operational source of truth
- projection data is exported to a warehouse-friendly shape
- QuickSight reads analytics-oriented datasets
- Amazon Q reads structured workflow and incident context through a controlled adapter

This keeps the architecture honest:

- core workflow logic stays in Go
- analytics live in QuickSight
- natural-language explanation lives behind a bounded interface

#### 7D. Why This Is Worth Doing Later

This AWS track is valuable because it teaches a second layer of the same problem:

- how operational systems feed reporting
- how AI sits on top of structured context
- how to separate authoritative control paths from exploratory and analytical tools

But it should stay off the critical path for v1.

## Design Rules

### Rule 1: No Fake CRUD Center

The project can have entities, but entity management is not the product.

The product is orchestration and recovery.

### Rule 2: Workflows First

Whenever there is a design choice, prefer the approach that highlights:

- long-running state
- retries
- waiting
- escalation
- recovery

### Rule 3: AI Assists, It Does Not Govern

AI may suggest, summarize, or classify.

Authoritative control stays in explicit workflow and policy logic.

### Rule 4: Preserve Operational Truth

Keep the difference between:

- incoming signals
- workflow decisions
- current projection state

### Rule 5: Simulate Real Pain

The demo scenarios should include:

- flaky systems
- conflicting data
- repeated failures
- human overrides

Otherwise the project will drift back into toy-software territory.

## Recommended V1 Scope

For v1, keep it tight:

- one product line
- three stations: assembly, test, inspection
- one workflow per unit
- retry policy for flaky test failures
- hold + escalation for repeated failure
- rework branch
- station heartbeat timeout
- unit timeline endpoint
- active incident dashboard endpoint
- rules-based triage engine

That is enough to be serious without becoming endless.

## Questions To Resolve Next

- Do we want to implement our own tiny orchestrator first, or start directly with Temporal?
- What exact workflow states should a unit move through in v1?
- What failure classes should be retryable vs terminal?
- What projections are the minimum useful ones?
- What should the first live demo scenario be?
