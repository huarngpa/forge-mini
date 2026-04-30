package orchestration

import (
	"context"

	"forge-mini/internal/signals"
	"forge-mini/internal/workflow"
)

type RuntimeName string

const (
	RuntimeInProcess RuntimeName = "inprocess"
	RuntimeTemporal  RuntimeName = "temporal"
)

type Runtime interface {
	HandleSignal(ctx context.Context, signal signals.OperationalSignal) (workflow.HandleResult, error)
	HandleTimer(ctx context.Context, timer workflow.TimerPayload) (workflow.HandleResult, error)
	RunScenario(ctx context.Context, name, unitID string) (ScenarioRunResult, error)
	GetWorkflow(ctx context.Context, unitID string) (workflow.UnitWorkflow, error)
}

type ScenarioRunResult struct {
	Scenario      string                `json:"scenario"`
	UnitID        string                `json:"unit_id"`
	Steps         []ScenarioStepResult  `json:"steps"`
	FinalWorkflow workflow.UnitWorkflow `json:"final_workflow"`
}

type ScenarioStepResult struct {
	Name   string                `json:"name"`
	Result workflow.HandleResult `json:"result"`
}
