package temporalruntime

const (
	DefaultNamespace = "default"
	DefaultHostPort  = "localhost:7233"
	DefaultTaskQueue = "forge-mini"

	UnitWorkflowName   = "ForgeMiniUnitWorkflow"
	SignalOperational  = "operational_signal"
	SignalManualTimer  = "manual_timer"
	QueryWorkflowState = "workflow_state"
)

type UnitWorkflowInput struct {
	UnitID string `json:"unit_id"`
}
