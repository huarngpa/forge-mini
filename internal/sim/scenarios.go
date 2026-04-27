package sim

import (
	"encoding/json"
	"fmt"
	"time"

	"forge-mini/internal/signals"
	"forge-mini/internal/workflow"
)

type Step struct {
	Name   string
	Signal *signals.OperationalSignal
	Timer  *workflow.TimerPayload
}

func ScenarioNames() []string {
	return []string{
		"happy_path_unit",
		"flaky_retry_success",
		"repeated_failure_rework",
		"heartbeat_timeout_hold",
		"duplicate_signal_rejected",
	}
}

func BuildScenario(name, unitID string, base time.Time) ([]Step, error) {
	switch name {
	case "happy_path_unit":
		return happyPathUnit(unitID, base), nil
	case "flaky_retry_success":
		return flakyRetrySuccess(unitID, base)
	case "repeated_failure_rework":
		return repeatedFailureRework(unitID, base)
	case "heartbeat_timeout_hold":
		return heartbeatTimeoutHold(unitID, base), nil
	case "duplicate_signal_rejected":
		return duplicateSignalRejected(unitID, base), nil
	default:
		return nil, fmt.Errorf("unknown scenario %q", name)
	}
}

func happyPathUnit(unitID string, base time.Time) []Step {
	return []Step{
		{Name: "admit", Signal: &signals.OperationalSignal{ID: unitID + "-sig-1", Type: signals.SignalUnitAdmitted, UnitID: unitID, Source: "planner", OccurredAt: base}},
		{Name: "enter-assembly", Signal: &signals.OperationalSignal{ID: unitID + "-sig-2", Type: signals.SignalStationEntered, UnitID: unitID, Source: "assembly-station-1", StationID: "assembly", OccurredAt: base.Add(1 * time.Second)}},
		{Name: "assembly-complete", Signal: &signals.OperationalSignal{ID: unitID + "-sig-3", Type: signals.SignalAssemblyCompleted, UnitID: unitID, Source: "assembly-station-1", StationID: "assembly", OccurredAt: base.Add(2 * time.Second)}},
		{Name: "test-started", Signal: &signals.OperationalSignal{ID: unitID + "-sig-4", Type: signals.SignalTestStarted, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(3 * time.Second)}},
		{Name: "test-passed", Signal: &signals.OperationalSignal{ID: unitID + "-sig-5", Type: signals.SignalTestPassed, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(4 * time.Second)}},
		{Name: "enter-inspection", Signal: &signals.OperationalSignal{ID: unitID + "-sig-6", Type: signals.SignalStationEntered, UnitID: unitID, Source: "inspection-station-1", StationID: "inspection", OccurredAt: base.Add(5 * time.Second)}},
		{Name: "inspection-complete", Signal: &signals.OperationalSignal{ID: unitID + "-sig-7", Type: signals.SignalInspectionCompleted, UnitID: unitID, Source: "inspection-station-1", StationID: "inspection", OccurredAt: base.Add(6 * time.Second)}},
	}
}

func flakyRetrySuccess(unitID string, base time.Time) ([]Step, error) {
	firstFailurePayload, err := json.Marshal(signals.TestFailedPayload{
		FailureCode:  "fixture_comm_error",
		FailureClass: "flaky_infra",
		Message:      "lost connection to fixture controller",
	})
	if err != nil {
		return nil, err
	}

	return []Step{
		{Name: "admit", Signal: &signals.OperationalSignal{ID: unitID + "-sig-1", Type: signals.SignalUnitAdmitted, UnitID: unitID, Source: "planner", OccurredAt: base}},
		{Name: "enter-assembly", Signal: &signals.OperationalSignal{ID: unitID + "-sig-2", Type: signals.SignalStationEntered, UnitID: unitID, Source: "assembly-station-1", StationID: "assembly", OccurredAt: base.Add(1 * time.Second)}},
		{Name: "assembly-complete", Signal: &signals.OperationalSignal{ID: unitID + "-sig-3", Type: signals.SignalAssemblyCompleted, UnitID: unitID, Source: "assembly-station-1", StationID: "assembly", OccurredAt: base.Add(2 * time.Second)}},
		{Name: "test-started", Signal: &signals.OperationalSignal{ID: unitID + "-sig-4", Type: signals.SignalTestStarted, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(3 * time.Second)}},
		{Name: "test-failed", Signal: &signals.OperationalSignal{ID: unitID + "-sig-5", Type: signals.SignalTestFailed, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(4 * time.Second), Payload: firstFailurePayload}},
		{Name: "retry-backoff-fired", Timer: &workflow.TimerPayload{Key: unitID + ":" + workflow.TimerRetryBackoff + ":test", WorkflowID: "wf-" + unitID, UnitID: unitID, TimerType: workflow.TimerRetryBackoff, StationID: "test", FireAt: base.Add(7 * time.Second)}},
		{Name: "test-passed", Signal: &signals.OperationalSignal{ID: unitID + "-sig-6", Type: signals.SignalTestPassed, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(8 * time.Second)}},
	}, nil
}

func repeatedFailureRework(unitID string, base time.Time) ([]Step, error) {
	failurePayload, err := json.Marshal(signals.TestFailedPayload{
		FailureCode:  "fixture_comm_error",
		FailureClass: "flaky_infra",
		Message:      "fixture communications still unstable after retry",
	})
	if err != nil {
		return nil, err
	}
	dispositionPayload, err := json.Marshal(signals.QualityDispositionPayload{
		Action: "route_to_rework",
		Reason: "reseat fixture connector and rerun sequence",
		Actor:  "eng-102",
	})
	if err != nil {
		return nil, err
	}

	return []Step{
		{Name: "admit", Signal: &signals.OperationalSignal{ID: unitID + "-sig-1", Type: signals.SignalUnitAdmitted, UnitID: unitID, Source: "planner", OccurredAt: base}},
		{Name: "enter-assembly", Signal: &signals.OperationalSignal{ID: unitID + "-sig-2", Type: signals.SignalStationEntered, UnitID: unitID, Source: "assembly-station-1", StationID: "assembly", OccurredAt: base.Add(1 * time.Second)}},
		{Name: "assembly-complete", Signal: &signals.OperationalSignal{ID: unitID + "-sig-3", Type: signals.SignalAssemblyCompleted, UnitID: unitID, Source: "assembly-station-1", StationID: "assembly", OccurredAt: base.Add(2 * time.Second)}},
		{Name: "test-started", Signal: &signals.OperationalSignal{ID: unitID + "-sig-4", Type: signals.SignalTestStarted, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(3 * time.Second)}},
		{Name: "test-failed-1", Signal: &signals.OperationalSignal{ID: unitID + "-sig-5", Type: signals.SignalTestFailed, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(4 * time.Second), Payload: failurePayload}},
		{Name: "retry-backoff-fired", Timer: &workflow.TimerPayload{Key: unitID + ":" + workflow.TimerRetryBackoff + ":test", WorkflowID: "wf-" + unitID, UnitID: unitID, TimerType: workflow.TimerRetryBackoff, StationID: "test", FireAt: base.Add(7 * time.Second)}},
		{Name: "test-failed-2", Signal: &signals.OperationalSignal{ID: unitID + "-sig-6", Type: signals.SignalTestFailed, UnitID: unitID, Source: "test-bench-2", StationID: "test", OccurredAt: base.Add(8 * time.Second), Payload: failurePayload}},
		{Name: "route-to-rework", Signal: &signals.OperationalSignal{ID: unitID + "-sig-7", Type: signals.SignalQualityDisposition, UnitID: unitID, Source: "quality-console", StationID: "test", OccurredAt: base.Add(9 * time.Second), Payload: dispositionPayload}},
		{Name: "rework-started", Signal: &signals.OperationalSignal{ID: unitID + "-sig-8", Type: signals.SignalReworkStarted, UnitID: unitID, Source: "rework-station-1", StationID: "rework", OccurredAt: base.Add(10 * time.Second)}},
		{Name: "rework-completed", Signal: &signals.OperationalSignal{ID: unitID + "-sig-9", Type: signals.SignalReworkCompleted, UnitID: unitID, Source: "rework-station-1", StationID: "rework", OccurredAt: base.Add(11 * time.Second)}},
	}, nil
}

func heartbeatTimeoutHold(unitID string, base time.Time) []Step {
	return []Step{
		{Name: "admit", Signal: &signals.OperationalSignal{ID: unitID + "-sig-1", Type: signals.SignalUnitAdmitted, UnitID: unitID, Source: "planner", OccurredAt: base}},
		{Name: "enter-assembly", Signal: &signals.OperationalSignal{ID: unitID + "-sig-2", Type: signals.SignalStationEntered, UnitID: unitID, Source: "assembly-station-1", StationID: "assembly", OccurredAt: base.Add(1 * time.Second)}},
		{Name: "heartbeat-timeout-fired", Timer: &workflow.TimerPayload{Key: unitID + ":" + workflow.TimerHeartbeatTimeout + ":assembly", WorkflowID: "wf-" + unitID, UnitID: unitID, TimerType: workflow.TimerHeartbeatTimeout, StationID: "assembly", FireAt: base.Add(31 * time.Second)}},
	}
}

func duplicateSignalRejected(unitID string, base time.Time) []Step {
	signal := signals.OperationalSignal{ID: unitID + "-sig-1", Type: signals.SignalUnitAdmitted, UnitID: unitID, Source: "planner", OccurredAt: base}
	return []Step{
		{Name: "admit", Signal: &signal},
		{Name: "duplicate-admit", Signal: &signal},
	}
}
