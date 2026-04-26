package workflow

import "time"

const (
	StatePending             = "pending"
	StateReadyForAssembly    = "ready_for_assembly"
	StateInAssembly          = "in_assembly"
	StateReadyForTest        = "ready_for_test"
	StateAwaitingTestResult  = "awaiting_test_result"
	StateRetryPending        = "retry_pending"
	StateOnHold              = "on_hold"
	StateAwaitingRework      = "awaiting_rework"
	StateInRework            = "in_rework"
	StateReadyForInspection  = "ready_for_inspection"
	StateInInspection        = "in_inspection"
	StateCompleted           = "completed"
	StateScrapped            = "scrapped"
	TimerTestResultTimeout   = "test_result_timeout"
	TimerHeartbeatTimeout    = "station_heartbeat_timeout"
	TimerRetryBackoff        = "retry_backoff_timer"
	TimerManualReviewSLA     = "manual_review_sla_timer"
)

type UnitWorkflow struct {
	WorkflowID            string         `json:"workflow_id"`
	UnitID                string         `json:"unit_id"`
	ProductCode           string         `json:"product_code"`
	State                 string         `json:"state"`
	CurrentStationID      string         `json:"current_station_id"`
	AttemptCountByStation map[string]int `json:"attempt_count_by_station"`
	ActiveTimerKeys       []string       `json:"active_timer_keys"`
	ActiveHoldReason      string         `json:"active_hold_reason"`
	OpenIncidentID        string         `json:"open_incident_id"`
	LastSignalID          string         `json:"last_signal_id"`
	LastSignalType        string         `json:"last_signal_type"`
	LastSignalAt          time.Time      `json:"last_signal_at"`
	LastDecisionAt        time.Time      `json:"last_decision_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

func New(unitID string, now time.Time) UnitWorkflow {
	return UnitWorkflow{
		WorkflowID:            "wf-" + unitID,
		UnitID:                unitID,
		State:                 StatePending,
		AttemptCountByStation: map[string]int{},
		ActiveTimerKeys:       []string{},
		UpdatedAt:             now,
	}
}

