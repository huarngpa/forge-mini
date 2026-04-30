package temporalruntime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"forge-mini/internal/orchestration"
	"forge-mini/internal/signals"
	"forge-mini/internal/sim"
	domainworkflow "forge-mini/internal/workflow"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

type Runtime struct {
	client    client.Client
	taskQueue string
	now       func() time.Time
}

func NewRuntime(temporalClient client.Client, taskQueue string) *Runtime {
	if taskQueue == "" {
		taskQueue = DefaultTaskQueue
	}
	return &Runtime{
		client:    temporalClient,
		taskQueue: taskQueue,
		now:       time.Now,
	}
}

func (r *Runtime) HandleSignal(ctx context.Context, signal signals.OperationalSignal) (domainworkflow.HandleResult, error) {
	if signal.ReceivedAt.IsZero() {
		signal.ReceivedAt = r.now()
	}
	workflowID := unitWorkflowID(signal.UnitID)
	options := client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                r.taskQueue,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		WorkflowExecutionTimeout: 24 * time.Hour,
	}
	_, err := r.client.SignalWithStartWorkflow(ctx, workflowID, SignalOperational, signal, options, UnitWorkflowName, UnitWorkflowInput{UnitID: signal.UnitID})
	if err != nil {
		return domainworkflow.HandleResult{}, err
	}
	wf, err := r.queryWorkflow(ctx, workflowID)
	if err != nil {
		return domainworkflow.HandleResult{}, err
	}
	return domainworkflow.HandleResult{
		Accepted:      true,
		WorkflowState: wf.State,
	}, nil
}

func (r *Runtime) HandleTimer(ctx context.Context, timer domainworkflow.TimerPayload) (domainworkflow.HandleResult, error) {
	workflowID := unitWorkflowID(timer.UnitID)
	if err := r.client.SignalWorkflow(ctx, workflowID, "", SignalManualTimer, timer); err != nil {
		return domainworkflow.HandleResult{}, err
	}
	wf, err := r.queryWorkflow(ctx, workflowID)
	if err != nil {
		return domainworkflow.HandleResult{}, err
	}
	return domainworkflow.HandleResult{
		Accepted:      true,
		WorkflowState: wf.State,
	}, nil
}

func (r *Runtime) RunScenario(ctx context.Context, name, unitID string) (orchestration.ScenarioRunResult, error) {
	steps, err := sim.BuildScenario(name, unitID, r.now())
	if err != nil {
		return orchestration.ScenarioRunResult{}, err
	}

	results := make([]orchestration.ScenarioStepResult, 0, len(steps))
	for _, step := range steps {
		var result domainworkflow.HandleResult
		switch {
		case step.Signal != nil:
			result, err = r.HandleSignal(ctx, *step.Signal)
		case step.Timer != nil:
			result, err = r.HandleTimer(ctx, *step.Timer)
		default:
			continue
		}
		if err != nil {
			return orchestration.ScenarioRunResult{}, fmt.Errorf("run step %q: %w", step.Name, err)
		}
		results = append(results, orchestration.ScenarioStepResult{
			Name:   step.Name,
			Result: result,
		})
	}

	wf, err := r.queryWorkflow(ctx, unitWorkflowID(unitID))
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return orchestration.ScenarioRunResult{
				Scenario: name,
				UnitID:   unitID,
				Steps:    results,
			}, nil
		}
		return orchestration.ScenarioRunResult{}, err
	}
	return orchestration.ScenarioRunResult{
		Scenario:      name,
		UnitID:        unitID,
		Steps:         results,
		FinalWorkflow: wf,
	}, nil
}

func (r *Runtime) GetWorkflow(ctx context.Context, unitID string) (domainworkflow.UnitWorkflow, error) {
	return r.queryWorkflow(ctx, unitWorkflowID(unitID))
}

func (r *Runtime) queryWorkflow(ctx context.Context, workflowID string) (domainworkflow.UnitWorkflow, error) {
	value, err := r.client.QueryWorkflow(ctx, workflowID, "", QueryWorkflowState)
	if err != nil {
		return domainworkflow.UnitWorkflow{}, err
	}
	var wf domainworkflow.UnitWorkflow
	if err := value.Get(&wf); err != nil {
		return domainworkflow.UnitWorkflow{}, err
	}
	return wf, nil
}

func unitWorkflowID(unitID string) string {
	return "forge-mini-unit-" + unitID
}
