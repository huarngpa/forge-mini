package workflow_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"forge-mini/internal/app"
	"forge-mini/internal/signals"
	"forge-mini/internal/workflow"
)

func TestFlakyFailureThenRetryThenPass(t *testing.T) {
	container := app.New()
	svc := container.Service
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	svcNow := now
	svcNowFn := func() time.Time {
		return svcNow
	}
	svcTestHookSetNow(svc, svcNowFn)

	ctx := context.Background()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "1", Type: signals.SignalUnitAdmitted, UnitID: "unit-1", Source: "planner", OccurredAt: now})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "2", Type: signals.SignalStationEntered, UnitID: "unit-1", Source: "assembly", StationID: "assembly", OccurredAt: now})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "3", Type: signals.SignalAssemblyCompleted, UnitID: "unit-1", Source: "assembly", StationID: "assembly", OccurredAt: now})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "4", Type: signals.SignalTestStarted, UnitID: "unit-1", Source: "test", StationID: "test", OccurredAt: now})

	payload, _ := json.Marshal(signals.TestFailedPayload{
		FailureCode:  "fixture_comm_error",
		FailureClass: "flaky_infra",
		Message:      "lost connection",
	})
	result, err := svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "5",
		Type:       signals.SignalTestFailed,
		UnitID:     "unit-1",
		Source:     "test",
		StationID:  "test",
		OccurredAt: now,
		Payload:    payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateRetryPending {
		t.Fatalf("expected retry_pending, got %s", result.WorkflowState)
	}

	svcNow = now.Add(3 * time.Second)
	result, err = svc.HandleTimer(ctx, workflow.TimerPayload{
		Key:        "unit-1:" + workflow.TimerRetryBackoff + ":test",
		WorkflowID: "wf-unit-1",
		UnitID:     "unit-1",
		TimerType:  workflow.TimerRetryBackoff,
		StationID:  "test",
		FireAt:     svcNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateAwaitingTestResult {
		t.Fatalf("expected awaiting_test_result, got %s", result.WorkflowState)
	}

	result, err = svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "6",
		Type:       signals.SignalTestPassed,
		UnitID:     "unit-1",
		Source:     "test",
		StationID:  "test",
		OccurredAt: svcNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateReadyForInspection {
		t.Fatalf("expected ready_for_inspection, got %s", result.WorkflowState)
	}
}

func TestRepeatedFailureThenRouteToRework(t *testing.T) {
	container := app.New()
	svc := container.Service
	now := time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC)
	svcNow := now
	svcNowFn := func() time.Time {
		return svcNow
	}
	svcTestHookSetNow(svc, svcNowFn)

	ctx := context.Background()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "a1", Type: signals.SignalUnitAdmitted, UnitID: "unit-2", Source: "planner", OccurredAt: now})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "a2", Type: signals.SignalStationEntered, UnitID: "unit-2", Source: "assembly", StationID: "assembly", OccurredAt: now})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "a3", Type: signals.SignalAssemblyCompleted, UnitID: "unit-2", Source: "assembly", StationID: "assembly", OccurredAt: now})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "a4", Type: signals.SignalTestStarted, UnitID: "unit-2", Source: "test", StationID: "test", OccurredAt: now})

	payload, _ := json.Marshal(signals.TestFailedPayload{
		FailureCode:  "fixture_comm_error",
		FailureClass: "flaky_infra",
		Message:      "fixture failed twice",
	})
	result, err := svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "a5",
		Type:       signals.SignalTestFailed,
		UnitID:     "unit-2",
		Source:     "test",
		StationID:  "test",
		OccurredAt: now,
		Payload:    payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateRetryPending {
		t.Fatalf("expected retry_pending, got %s", result.WorkflowState)
	}

	svcNow = now.Add(3 * time.Second)
	_, err = svc.HandleTimer(ctx, workflow.TimerPayload{
		Key:        "unit-2:" + workflow.TimerRetryBackoff + ":test",
		WorkflowID: "wf-unit-2",
		UnitID:     "unit-2",
		TimerType:  workflow.TimerRetryBackoff,
		StationID:  "test",
		FireAt:     svcNow,
	})
	if err != nil {
		t.Fatal(err)
	}

	svcNow = now.Add(4 * time.Second)
	result, err = svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "a6",
		Type:       signals.SignalTestFailed,
		UnitID:     "unit-2",
		Source:     "test",
		StationID:  "test",
		OccurredAt: svcNow,
		Payload:    payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateOnHold {
		t.Fatalf("expected on_hold, got %s", result.WorkflowState)
	}

	disposition, _ := json.Marshal(signals.QualityDispositionPayload{
		Action: "route_to_rework",
		Reason: "manual fixture reseat required",
		Actor:  "eng-102",
	})
	result, err = svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "a7",
		Type:       signals.SignalQualityDisposition,
		UnitID:     "unit-2",
		Source:     "quality-console",
		StationID:  "test",
		OccurredAt: svcNow.Add(time.Second),
		Payload:    disposition,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateAwaitingRework {
		t.Fatalf("expected awaiting_rework, got %s", result.WorkflowState)
	}

	result, err = svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "a8",
		Type:       signals.SignalReworkStarted,
		UnitID:     "unit-2",
		Source:     "rework",
		StationID:  "rework",
		OccurredAt: svcNow.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateInRework {
		t.Fatalf("expected in_rework, got %s", result.WorkflowState)
	}

	result, err = svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "a9",
		Type:       signals.SignalReworkCompleted,
		UnitID:     "unit-2",
		Source:     "rework",
		StationID:  "rework",
		OccurredAt: svcNow.Add(3 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateReadyForTest {
		t.Fatalf("expected ready_for_test, got %s", result.WorkflowState)
	}
}

func runSignal(t *testing.T, svc *app.Service, ctx context.Context, signal signals.OperationalSignal) {
	t.Helper()
	if _, err := svc.HandleSignal(ctx, signal); err != nil {
		t.Fatal(err)
	}
}

func svcTestHookSetNow(svc *app.Service, now func() time.Time) {
	svc.SetNow(now)
}
