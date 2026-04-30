package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"forge-mini/internal/incidents"
	"forge-mini/internal/orchestration"
	"forge-mini/internal/policy"
	"forge-mini/internal/projections"
	"forge-mini/internal/signals"
	"forge-mini/internal/sim"
	"forge-mini/internal/store"
	"forge-mini/internal/temporalruntime"
	"forge-mini/internal/triage"
	"forge-mini/internal/workflow"
	"go.temporal.io/sdk/client"
)

type Container struct {
	Memory  *store.Memory
	Engine  workflow.TransitionEngine
	Runtime orchestration.Runtime
	Service *Service
	Triage  *triage.Service
	Worker  *temporalruntime.WorkerHandle
	Client  client.Client
}

func New() *Container {
	container, err := NewWithRuntime(orchestration.RuntimeInProcess)
	if err != nil {
		panic(err)
	}
	return container
}

func NewFromEnv() (*Container, error) {
	name := orchestration.RuntimeName(os.Getenv("FORGE_RUNTIME"))
	if name == "" {
		name = orchestration.RuntimeInProcess
	}
	return NewWithRuntime(name)
}

func NewWithRuntime(name orchestration.RuntimeName) (*Container, error) {
	mem := store.NewMemory()
	engine := workflow.NewEngine(policy.DefaultRetryPolicy{})
	svc := &Service{
		memory: mem,
		engine: engine,
		now:    time.Now,
	}
	triageSvc := triage.New(mem, mem)
	var runtime orchestration.Runtime
	switch name {
	case orchestration.RuntimeInProcess:
		runtime = svc
	case orchestration.RuntimeTemporal:
		hostPort := envOrDefault("TEMPORAL_HOST_PORT", temporalruntime.DefaultHostPort)
		namespace := envOrDefault("TEMPORAL_NAMESPACE", temporalruntime.DefaultNamespace)
		taskQueue := envOrDefault("TEMPORAL_TASK_QUEUE", temporalruntime.DefaultTaskQueue)
		temporalClient, err := client.Dial(client.Options{
			HostPort:  hostPort,
			Namespace: namespace,
		})
		if err != nil {
			return nil, fmt.Errorf("connect temporal: %w", err)
		}
		workerHandle, err := temporalruntime.StartWorker(temporalClient, taskQueue)
		if err != nil {
			temporalClient.Close()
			return nil, fmt.Errorf("start temporal worker: %w", err)
		}
		runtime = temporalruntime.NewRuntime(temporalClient, taskQueue)
		return &Container{
			Memory:  mem,
			Engine:  engine,
			Runtime: runtime,
			Service: svc,
			Triage:  triageSvc,
			Worker:  workerHandle,
			Client:  temporalClient,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported runtime %q", name)
	}
	return &Container{
		Memory:  mem,
		Engine:  engine,
		Runtime: runtime,
		Service: svc,
		Triage:  triageSvc,
	}, nil
}

func (c *Container) Close() {
	if c == nil {
		return
	}
	c.Worker.Stop()
	if c.Client != nil {
		c.Client.Close()
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

type Service struct {
	memory *store.Memory
	engine workflow.TransitionEngine
	now    func() time.Time
}

func (s *Service) HandleSignal(ctx context.Context, signal signals.OperationalSignal) (workflow.HandleResult, error) {
	if signal.ReceivedAt.IsZero() {
		signal.ReceivedAt = s.now()
	}

	exists, err := s.memory.SignalExists(ctx, signal.ID)
	if err != nil {
		return workflow.HandleResult{}, err
	}
	if exists {
		return workflow.HandleResult{
			Accepted:      false,
			WorkflowState: "",
			DecisionIDs:   nil,
		}, nil
	}
	if err := s.memory.AppendSignal(ctx, signal); err != nil {
		return workflow.HandleResult{}, err
	}

	snapshot, err := s.memory.GetWorkflow(ctx, signal.UnitID)
	if err != nil && err != store.ErrNotFound {
		return workflow.HandleResult{}, err
	}
	if err == store.ErrNotFound {
		snapshot = workflow.New(signal.UnitID, s.now())
	}

	result, err := s.engine.ApplySignal(snapshot, signal, s.now())
	if err != nil {
		return workflow.HandleResult{}, err
	}

	return s.commitTransition(ctx, signal.UnitID, result)
}

func (s *Service) HandleTimer(ctx context.Context, timer workflow.TimerPayload) (workflow.HandleResult, error) {
	snapshot, err := s.memory.GetWorkflow(ctx, timer.UnitID)
	if err != nil {
		return workflow.HandleResult{}, err
	}
	result, err := s.engine.ApplyTimer(snapshot, timer, s.now())
	if err != nil {
		return workflow.HandleResult{}, err
	}
	return s.commitTransition(ctx, timer.UnitID, result)
}

func (s *Service) RunScenario(ctx context.Context, name, unitID string) (orchestration.ScenarioRunResult, error) {
	base := s.now()
	steps, err := sim.BuildScenario(name, unitID, base)
	if err != nil {
		return orchestration.ScenarioRunResult{}, err
	}

	results := make([]orchestration.ScenarioStepResult, 0, len(steps))
	for _, step := range steps {
		var handleResult workflow.HandleResult
		switch {
		case step.Signal != nil:
			handleResult, err = s.HandleSignal(ctx, *step.Signal)
		case step.Timer != nil:
			handleResult, err = s.HandleTimer(ctx, *step.Timer)
		default:
			continue
		}
		if err != nil {
			return orchestration.ScenarioRunResult{}, fmt.Errorf("run step %q: %w", step.Name, err)
		}
		results = append(results, orchestration.ScenarioStepResult{
			Name:   step.Name,
			Result: handleResult,
		})
	}

	finalWorkflow, err := s.memory.GetWorkflow(ctx, unitID)
	if err != nil {
		return orchestration.ScenarioRunResult{}, err
	}

	return orchestration.ScenarioRunResult{
		Scenario:      name,
		UnitID:        unitID,
		Steps:         results,
		FinalWorkflow: finalWorkflow,
	}, nil
}

func (s *Service) GetWorkflow(ctx context.Context, unitID string) (workflow.UnitWorkflow, error) {
	return s.memory.GetWorkflow(ctx, unitID)
}

func (s *Service) commitTransition(ctx context.Context, unitID string, result workflow.TransitionResult) (workflow.HandleResult, error) {
	decisionIDs := make([]string, 0, len(result.Decisions))
	for _, decision := range result.Decisions {
		if err := s.memory.AppendDecision(ctx, decision); err != nil {
			return workflow.HandleResult{}, err
		}
		decisionIDs = append(decisionIDs, decision.ID)
	}

	for _, key := range result.TimersToCancel {
		if err := s.memory.Cancel(ctx, key); err != nil {
			return workflow.HandleResult{}, err
		}
	}
	for _, req := range result.TimersToStart {
		if err := s.memory.Schedule(ctx, req.Key, req.FireAt, workflow.TimerPayload{
			Key:        req.Key,
			WorkflowID: result.NextWorkflow.WorkflowID,
			UnitID:     result.NextWorkflow.UnitID,
			TimerType:  req.TimerType,
			StationID:  req.StationID,
		}); err != nil {
			return workflow.HandleResult{}, err
		}
	}

	if result.IncidentAction.Type != workflow.IncidentNone {
		if err := s.applyIncidentAction(ctx, result.NextWorkflow, result.IncidentAction); err != nil {
			return workflow.HandleResult{}, err
		}
	}

	previousIncidentID := result.NextWorkflow.OpenIncidentID
	incident, err := s.memory.GetOpenIncidentByUnit(ctx, unitID)
	if err != nil {
		return workflow.HandleResult{}, err
	}
	if incident != nil {
		result.NextWorkflow.OpenIncidentID = incident.ID
	} else {
		if previousIncidentID != "" {
			_ = s.memory.DeleteIncidentView(ctx, previousIncidentID)
		}
		result.NextWorkflow.OpenIncidentID = ""
	}

	if err := s.memory.PutWorkflow(ctx, result.NextWorkflow); err != nil {
		return workflow.HandleResult{}, err
	}
	if err := s.rebuildProjections(ctx, result.NextWorkflow); err != nil {
		return workflow.HandleResult{}, err
	}

	return workflow.HandleResult{
		Accepted:      true,
		WorkflowState: result.NextWorkflow.State,
		DecisionIDs:   decisionIDs,
		IncidentID:    result.NextWorkflow.OpenIncidentID,
	}, nil
}

func (s *Service) applyIncidentAction(ctx context.Context, wf workflow.UnitWorkflow, action workflow.IncidentAction) error {
	existing, err := s.memory.GetOpenIncidentByUnit(ctx, wf.UnitID)
	if err != nil {
		return err
	}

	switch action.Type {
	case workflow.IncidentUpsert:
		if existing == nil {
			existing = &incidents.Incident{
				ID:         fmt.Sprintf("inc-%d", s.now().UnixNano()),
				UnitID:     wf.UnitID,
				WorkflowID: wf.WorkflowID,
				OpenedAt:   s.now(),
			}
		}
		existing.StationID = action.StationID
		existing.Status = incidents.StatusOpen
		existing.Category = action.Category
		existing.Severity = action.Severity
		existing.Summary = action.Summary
		existing.RecommendedAction = action.RecommendedAction
		existing.LastUpdatedAt = s.now()
		return s.memory.PutIncident(ctx, *existing)
	case workflow.IncidentClose:
		if existing == nil {
			return nil
		}
		existing.Status = incidents.StatusResolved
		existing.LastUpdatedAt = s.now()
		return s.memory.PutIncident(ctx, *existing)
	default:
		return nil
	}
}

func (s *Service) rebuildProjections(ctx context.Context, wf workflow.UnitWorkflow) error {
	view := projections.UnitStatusView{
		UnitID:           wf.UnitID,
		WorkflowState:    wf.State,
		CurrentStationID: wf.CurrentStationID,
		ActiveHoldReason: wf.ActiveHoldReason,
		OpenIncidentID:   wf.OpenIncidentID,
		RetryCount:       wf.AttemptCountByStation["test"],
		LastSignalType:   wf.LastSignalType,
		LastUpdatedAt:    wf.UpdatedAt,
	}
	decisions, err := s.memory.ListDecisionsByWorkflow(ctx, wf.WorkflowID)
	if err != nil {
		return err
	}
	if len(decisions) > 0 {
		view.LastDecisionType = string(decisions[len(decisions)-1].DecisionType)
	}
	if err := s.memory.PutUnitStatus(ctx, view); err != nil {
		return err
	}

	incident, err := s.memory.GetOpenIncidentByUnit(ctx, wf.UnitID)
	if err != nil {
		return err
	}
	if incident != nil {
		if err := s.memory.PutIncidentView(ctx, projections.IncidentDashboardItem{
			IncidentID:        incident.ID,
			UnitID:            incident.UnitID,
			StationID:         incident.StationID,
			Status:            string(incident.Status),
			Category:          incident.Category,
			Severity:          incident.Severity,
			RecommendedAction: incident.RecommendedAction,
			UpdatedAt:         incident.LastUpdatedAt,
		}); err != nil {
			return err
		}
	} else if wf.OpenIncidentID != "" {
		_ = s.memory.DeleteIncidentView(ctx, wf.OpenIncidentID)
	}

	status := "idle"
	if wf.State == workflow.StateOnHold {
		status = "blocked"
	} else if wf.CurrentStationID != "" {
		status = "active"
	}
	if wf.CurrentStationID != "" {
		return s.memory.PutStationHealth(ctx, projections.StationHealthView{
			StationID:          wf.CurrentStationID,
			ActiveUnitID:       wf.UnitID,
			CurrentStatus:      status,
			OpenIncidentCount:  boolToInt(wf.OpenIncidentID != ""),
			RetryCountLastHour: wf.AttemptCountByStation["test"],
			LastHeartbeatAt:    s.now(),
		})
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
