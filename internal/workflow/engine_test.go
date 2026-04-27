package workflow_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"forge-mini/internal/app"
	"forge-mini/internal/signals"
	"forge-mini/internal/store"
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

func TestHappyPathCompletesThroughInspection(t *testing.T) {
	container, svc, ctx, _, base := newHarness()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "hp1", Type: signals.SignalUnitAdmitted, UnitID: "happy-unit", Source: "planner", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "hp2", Type: signals.SignalStationEntered, UnitID: "happy-unit", Source: "assembly", StationID: "assembly", OccurredAt: base.Add(time.Second)})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "hp3", Type: signals.SignalAssemblyCompleted, UnitID: "happy-unit", Source: "assembly", StationID: "assembly", OccurredAt: base.Add(2 * time.Second)})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "hp4", Type: signals.SignalTestStarted, UnitID: "happy-unit", Source: "test", StationID: "test", OccurredAt: base.Add(3 * time.Second)})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "hp5", Type: signals.SignalTestPassed, UnitID: "happy-unit", Source: "test", StationID: "test", OccurredAt: base.Add(4 * time.Second)})

	result, err := svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "hp6",
		Type:       signals.SignalStationEntered,
		UnitID:     "happy-unit",
		Source:     "inspection",
		StationID:  "inspection",
		OccurredAt: base.Add(5 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateInInspection {
		t.Fatalf("expected in_inspection, got %s", result.WorkflowState)
	}

	result, err = svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "hp7",
		Type:       signals.SignalInspectionCompleted,
		UnitID:     "happy-unit",
		Source:     "inspection",
		StationID:  "inspection",
		OccurredAt: base.Add(6 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateCompleted {
		t.Fatalf("expected completed, got %s", result.WorkflowState)
	}

	timers, err := container.Memory.ListTimers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, timer := range timers {
		if timer.UnitID == "happy-unit" {
			t.Fatalf("expected completed unit to have no active timers, got %#v", timer)
		}
	}
}

func TestInspectionCompletedBeforeEntryIsRejected(t *testing.T) {
	container, svc, ctx, _, base := newHarness()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "insp1", Type: signals.SignalUnitAdmitted, UnitID: "inspection-unit", Source: "planner", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "insp2", Type: signals.SignalStationEntered, UnitID: "inspection-unit", Source: "assembly", StationID: "assembly", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "insp3", Type: signals.SignalAssemblyCompleted, UnitID: "inspection-unit", Source: "assembly", StationID: "assembly", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "insp4", Type: signals.SignalTestStarted, UnitID: "inspection-unit", Source: "test", StationID: "test", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "insp5", Type: signals.SignalTestPassed, UnitID: "inspection-unit", Source: "test", StationID: "test", OccurredAt: base})

	result, err := svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:         "insp6",
		Type:       signals.SignalInspectionCompleted,
		UnitID:     "inspection-unit",
		Source:     "inspection",
		StationID:  "inspection",
		OccurredAt: base.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateReadyForInspection {
		t.Fatalf("expected ready_for_inspection to remain unchanged, got %s", result.WorkflowState)
	}

	wf, err := container.Memory.GetWorkflow(ctx, "inspection-unit")
	if err != nil {
		t.Fatal(err)
	}
	decisions, err := container.Memory.ListDecisionsByWorkflow(ctx, wf.WorkflowID)
	if err != nil {
		t.Fatal(err)
	}
	if decisions[len(decisions)-1].DecisionType != workflow.DecisionRejectSignal {
		t.Fatalf("expected premature inspection completion to be rejected, got %s", decisions[len(decisions)-1].DecisionType)
	}
}

func TestUnexpectedSignalRejectedWithoutStateChange(t *testing.T) {
	container, svc, ctx, _, _ := newHarness()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "r1", Type: signals.SignalUnitAdmitted, UnitID: "reject-unit", Source: "planner"})
	result, err := svc.HandleSignal(ctx, signals.OperationalSignal{
		ID:        "r2",
		Type:      signals.SignalTestPassed,
		UnitID:    "reject-unit",
		Source:    "test",
		StationID: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateReadyForAssembly {
		t.Fatalf("expected state to remain ready_for_assembly, got %s", result.WorkflowState)
	}

	wf, err := container.Memory.GetWorkflow(ctx, "reject-unit")
	if err != nil {
		t.Fatal(err)
	}
	if wf.State != workflow.StateReadyForAssembly {
		t.Fatalf("expected workflow snapshot to remain ready_for_assembly, got %s", wf.State)
	}

	decisions, err := container.Memory.ListDecisionsByWorkflow(ctx, wf.WorkflowID)
	if err != nil {
		t.Fatal(err)
	}
	last := decisions[len(decisions)-1]
	if last.DecisionType != workflow.DecisionRejectSignal {
		t.Fatalf("expected last decision to be reject_signal, got %s", last.DecisionType)
	}
	if last.ReasonCode != "unexpected_signal_for_state" {
		t.Fatalf("expected unexpected_signal_for_state, got %s", last.ReasonCode)
	}
}

func TestHeartbeatTimeoutPutsWorkflowOnHoldAndOpensIncident(t *testing.T) {
	container, svc, ctx, _, base := newHarness()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "h1", Type: signals.SignalUnitAdmitted, UnitID: "heartbeat-unit", Source: "planner", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "h2", Type: signals.SignalStationEntered, UnitID: "heartbeat-unit", Source: "assembly", StationID: "assembly", OccurredAt: base.Add(time.Second)})

	result, err := svc.HandleTimer(ctx, workflow.TimerPayload{
		Key:        "heartbeat-unit:" + workflow.TimerHeartbeatTimeout + ":assembly",
		WorkflowID: "wf-heartbeat-unit",
		UnitID:     "heartbeat-unit",
		TimerType:  workflow.TimerHeartbeatTimeout,
		StationID:  "assembly",
		FireAt:     base.Add(31 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateOnHold {
		t.Fatalf("expected on_hold after heartbeat timeout, got %s", result.WorkflowState)
	}

	incident, err := container.Memory.GetOpenIncidentByUnit(ctx, "heartbeat-unit")
	if err != nil {
		t.Fatal(err)
	}
	if incident == nil || incident.Category != "heartbeat_timeout" {
		t.Fatalf("expected heartbeat_timeout incident, got %#v", incident)
	}

	view, err := container.Memory.GetUnitStatus(ctx, "heartbeat-unit")
	if err != nil {
		t.Fatal(err)
	}
	if view.WorkflowState != workflow.StateOnHold || view.ActiveHoldReason != workflow.TimerHeartbeatTimeout {
		t.Fatalf("unexpected unit view after heartbeat timeout: %#v", view)
	}

	incidentViews, err := container.Memory.ListIncidentViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidentViews) != 1 {
		t.Fatalf("expected 1 incident dashboard item, got %d", len(incidentViews))
	}
}

func TestTestResultTimeoutPutsWorkflowOnHoldAndOpensIncident(t *testing.T) {
	container, svc, ctx, _, base := newHarness()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "t1", Type: signals.SignalUnitAdmitted, UnitID: "timeout-unit", Source: "planner", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "t2", Type: signals.SignalStationEntered, UnitID: "timeout-unit", Source: "assembly", StationID: "assembly", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "t3", Type: signals.SignalAssemblyCompleted, UnitID: "timeout-unit", Source: "assembly", StationID: "assembly", OccurredAt: base})
	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "t4", Type: signals.SignalTestStarted, UnitID: "timeout-unit", Source: "test", StationID: "test", OccurredAt: base})

	result, err := svc.HandleTimer(ctx, workflow.TimerPayload{
		Key:        "timeout-unit:" + workflow.TimerTestResultTimeout + ":test",
		WorkflowID: "wf-timeout-unit",
		UnitID:     "timeout-unit",
		TimerType:  workflow.TimerTestResultTimeout,
		StationID:  "test",
		FireAt:     base.Add(46 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowState != workflow.StateOnHold {
		t.Fatalf("expected on_hold after test result timeout, got %s", result.WorkflowState)
	}

	incident, err := container.Memory.GetOpenIncidentByUnit(ctx, "timeout-unit")
	if err != nil {
		t.Fatal(err)
	}
	if incident == nil || incident.Category != "heartbeat_timeout" {
		t.Fatalf("expected timeout-driven incident, got %#v", incident)
	}
}

func TestNonRetryableFailuresImmediatelyHold(t *testing.T) {
	cases := []string{"hard_failure", "quality_suspect", "unknown"}
	for _, failureClass := range cases {
		t.Run(failureClass, func(t *testing.T) {
			container, svc, ctx, _, base := newHarness()
			unitID := "nonretry-" + failureClass

			runSignal(t, svc, ctx, signals.OperationalSignal{ID: unitID + "-1", Type: signals.SignalUnitAdmitted, UnitID: unitID, Source: "planner", OccurredAt: base})
			runSignal(t, svc, ctx, signals.OperationalSignal{ID: unitID + "-2", Type: signals.SignalStationEntered, UnitID: unitID, Source: "assembly", StationID: "assembly", OccurredAt: base})
			runSignal(t, svc, ctx, signals.OperationalSignal{ID: unitID + "-3", Type: signals.SignalAssemblyCompleted, UnitID: unitID, Source: "assembly", StationID: "assembly", OccurredAt: base})
			runSignal(t, svc, ctx, signals.OperationalSignal{ID: unitID + "-4", Type: signals.SignalTestStarted, UnitID: unitID, Source: "test", StationID: "test", OccurredAt: base})

			payload, _ := json.Marshal(signals.TestFailedPayload{
				FailureCode:  "fatal_error",
				FailureClass: failureClass,
				Message:      "non-retryable failure",
			})
			result, err := svc.HandleSignal(ctx, signals.OperationalSignal{
				ID:         unitID + "-5",
				Type:       signals.SignalTestFailed,
				UnitID:     unitID,
				Source:     "test",
				StationID:  "test",
				OccurredAt: base,
				Payload:    payload,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.WorkflowState != workflow.StateOnHold {
				t.Fatalf("expected immediate hold for %s, got %s", failureClass, result.WorkflowState)
			}

			wf, err := container.Memory.GetWorkflow(ctx, unitID)
			if err != nil {
				t.Fatal(err)
			}
			if wf.ActiveHoldReason != "retry_not_allowed" {
				t.Fatalf("expected retry_not_allowed hold reason, got %s", wf.ActiveHoldReason)
			}

			timers, err := container.Memory.ListTimers(ctx)
			if err != nil {
				t.Fatal(err)
			}
			foundRetryTimer := false
			for _, timer := range timers {
				if timer.UnitID == unitID && timer.TimerType == workflow.TimerRetryBackoff {
					foundRetryTimer = true
				}
			}
			if foundRetryTimer {
				t.Fatalf("did not expect retry timer for %s", failureClass)
			}
		})
	}
}

func TestDuplicateSignalIsIgnoredByService(t *testing.T) {
	container, svc, ctx, _, base := newHarness()

	first := signals.OperationalSignal{ID: "dup-1", Type: signals.SignalUnitAdmitted, UnitID: "dup-unit", Source: "planner", OccurredAt: base}
	result, err := svc.HandleSignal(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Accepted {
		t.Fatal("expected first signal to be accepted")
	}

	dupResult, err := svc.HandleSignal(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if dupResult.Accepted {
		t.Fatal("expected duplicate signal to be ignored")
	}

	wf, err := container.Memory.GetWorkflow(ctx, "dup-unit")
	if err != nil {
		t.Fatal(err)
	}
	decisions, err := container.Memory.ListDecisionsByWorkflow(ctx, wf.WorkflowID)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 2 {
		t.Fatalf("expected only the original 2 decisions, got %d", len(decisions))
	}
}

func TestScenarioRunnerFinalStates(t *testing.T) {
	cases := []struct {
		name          string
		unitID        string
		expectedState string
	}{
		{name: "happy_path_unit", unitID: "scenario-unit-0", expectedState: workflow.StateCompleted},
		{name: "flaky_retry_success", unitID: "scenario-unit-1", expectedState: workflow.StateReadyForInspection},
		{name: "repeated_failure_rework", unitID: "scenario-unit-2", expectedState: workflow.StateReadyForTest},
		{name: "heartbeat_timeout_hold", unitID: "scenario-unit-3", expectedState: workflow.StateOnHold},
		{name: "duplicate_signal_rejected", unitID: "scenario-unit-4", expectedState: workflow.StateReadyForAssembly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, svc, ctx, _, _ := newHarness()
			result, err := svc.RunScenario(ctx, tc.name, tc.unitID)
			if err != nil {
				t.Fatal(err)
			}
			if result.FinalWorkflow.State != tc.expectedState {
				t.Fatalf("expected final state %s, got %s", tc.expectedState, result.FinalWorkflow.State)
			}
			if len(result.Steps) == 0 {
				t.Fatal("expected scenario to produce steps")
			}
		})
	}
}

func TestResolvedIncidentProjectionIsRemoved(t *testing.T) {
	container, svc, ctx, _, _ := newHarness()

	result, err := svc.RunScenario(ctx, "flaky_retry_success", "resolved-unit")
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalWorkflow.State != workflow.StateReadyForInspection {
		t.Fatalf("expected ready_for_inspection, got %s", result.FinalWorkflow.State)
	}

	incident, err := container.Memory.GetOpenIncidentByUnit(ctx, "resolved-unit")
	if err != nil {
		t.Fatal(err)
	}
	if incident != nil {
		t.Fatalf("expected no open incident, got %#v", incident)
	}

	incidentViews, err := container.Memory.ListIncidentViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidentViews) != 0 {
		t.Fatalf("expected no incident dashboard items after resolution, got %d", len(incidentViews))
	}
}

func TestRejectedSignalStillPersistsRawSignalHistory(t *testing.T) {
	container, svc, ctx, _, _ := newHarness()

	runSignal(t, svc, ctx, signals.OperationalSignal{ID: "hist-1", Type: signals.SignalUnitAdmitted, UnitID: "hist-unit", Source: "planner"})
	_, err := svc.HandleSignal(ctx, signals.OperationalSignal{ID: "hist-2", Type: signals.SignalReworkCompleted, UnitID: "hist-unit", Source: "rework", StationID: "rework"})
	if err != nil {
		t.Fatal(err)
	}

	rawSignals, err := container.Memory.ListSignalsByUnit(ctx, "hist-unit")
	if err != nil {
		t.Fatal(err)
	}
	if len(rawSignals) != 2 {
		t.Fatalf("expected rejected signal to still be present in raw history, got %d signals", len(rawSignals))
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

func newHarness() (*app.Container, *app.Service, context.Context, *time.Time, time.Time) {
	container := app.New()
	svc := container.Service
	base := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	current := base
	svcTestHookSetNow(svc, func() time.Time {
		return current
	})
	return container, svc, context.Background(), &current, base
}

func TestStoreNotFoundSentinelStaysReachable(t *testing.T) {
	if store.ErrNotFound == nil {
		t.Fatal("expected ErrNotFound sentinel")
	}
}
