package temporalruntime

import (
	"encoding/json"
	"testing"
	"time"

	"forge-mini/internal/signals"
	domainworkflow "forge-mini/internal/workflow"
	"go.temporal.io/sdk/testsuite"
)

func TestUnitWorkflowCompletesHappyPathWithRetry(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(UnitWorkflow)

	base := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	unitID := "temporal-unit"
	failurePayload, err := json.Marshal(signals.TestFailedPayload{
		FailureCode:  "fixture_comm_error",
		FailureClass: "flaky_infra",
		Message:      "lost connection to fixture controller",
	})
	if err != nil {
		t.Fatal(err)
	}

	signalAt := func(delay time.Duration, signal signals.OperationalSignal) {
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(SignalOperational, signal)
		}, delay)
	}
	signalAt(time.Second, signals.OperationalSignal{ID: "tw-1", Type: signals.SignalUnitAdmitted, UnitID: unitID, Source: "planner", OccurredAt: base})
	signalAt(2*time.Second, signals.OperationalSignal{ID: "tw-2", Type: signals.SignalStationEntered, UnitID: unitID, Source: "assembly", StationID: "assembly", OccurredAt: base.Add(time.Second)})
	signalAt(3*time.Second, signals.OperationalSignal{ID: "tw-3", Type: signals.SignalAssemblyCompleted, UnitID: unitID, Source: "assembly", StationID: "assembly", OccurredAt: base.Add(2 * time.Second)})
	signalAt(4*time.Second, signals.OperationalSignal{ID: "tw-4", Type: signals.SignalTestStarted, UnitID: unitID, Source: "test", StationID: "test", OccurredAt: base.Add(3 * time.Second)})
	signalAt(5*time.Second, signals.OperationalSignal{ID: "tw-5", Type: signals.SignalTestFailed, UnitID: unitID, Source: "test", StationID: "test", OccurredAt: base.Add(4 * time.Second), Payload: failurePayload})
	signalAt(8*time.Second, signals.OperationalSignal{ID: "tw-6", Type: signals.SignalTestPassed, UnitID: unitID, Source: "test", StationID: "test", OccurredAt: base.Add(8 * time.Second)})
	signalAt(9*time.Second, signals.OperationalSignal{ID: "tw-7", Type: signals.SignalStationEntered, UnitID: unitID, Source: "inspection", StationID: "inspection", OccurredAt: base.Add(9 * time.Second)})
	signalAt(10*time.Second, signals.OperationalSignal{ID: "tw-8", Type: signals.SignalInspectionCompleted, UnitID: unitID, Source: "inspection", StationID: "inspection", OccurredAt: base.Add(10 * time.Second)})

	env.ExecuteWorkflow(UnitWorkflow, UnitWorkflowInput{UnitID: unitID})
	if !env.IsWorkflowCompleted() {
		t.Fatal("expected workflow to complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}

	var wf domainworkflow.UnitWorkflow
	if err := env.GetWorkflowResult(&wf); err != nil {
		t.Fatal(err)
	}
	if wf.State != domainworkflow.StateCompleted {
		t.Fatalf("expected completed, got %s", wf.State)
	}
	if wf.AttemptCountByStation["test"] != 1 {
		t.Fatalf("expected one failed test attempt, got %#v", wf.AttemptCountByStation)
	}
}
