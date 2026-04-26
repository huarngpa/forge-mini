package workflow

import "time"

type DecisionType string

const (
	DecisionAcceptSignal     DecisionType = "accept_signal"
	DecisionRejectSignal     DecisionType = "reject_signal"
	DecisionTransitionState  DecisionType = "transition_state"
	DecisionScheduleTimer    DecisionType = "schedule_timer"
	DecisionCancelTimer      DecisionType = "cancel_timer"
	DecisionStartRetry       DecisionType = "start_retry"
	DecisionEscalate         DecisionType = "escalate"
	DecisionPlaceHold        DecisionType = "place_hold"
	DecisionRouteToRework    DecisionType = "route_to_rework"
	DecisionCompleteIncident DecisionType = "complete_incident"
)

type WorkflowDecision struct {
	ID           string         `json:"id"`
	WorkflowID   string         `json:"workflow_id"`
	UnitID       string         `json:"unit_id"`
	DecisionType DecisionType   `json:"decision_type"`
	ReasonCode   string         `json:"reason_code"`
	Summary      string         `json:"summary"`
	TriggeredBy  string         `json:"triggered_by"`
	At           time.Time      `json:"at"`
	Metadata     map[string]any `json:"metadata"`
}

type TimerPayload struct {
	Key        string    `json:"key"`
	WorkflowID string    `json:"workflow_id"`
	UnitID     string    `json:"unit_id"`
	TimerType  string    `json:"timer_type"`
	StationID  string    `json:"station_id"`
	FireAt     time.Time `json:"fire_at"`
}

type TimerRequest struct {
	Key       string
	TimerType string
	StationID string
	FireAt    time.Time
}

type HandleResult struct {
	Accepted      bool     `json:"accepted"`
	WorkflowState string   `json:"workflow_state"`
	DecisionIDs   []string `json:"decision_ids"`
	IncidentID    string   `json:"incident_id,omitempty"`
}

type TransitionResult struct {
	NextWorkflow   UnitWorkflow
	Decisions      []WorkflowDecision
	IncidentAction IncidentAction
	TimersToStart  []TimerRequest
	TimersToCancel []string
}

type IncidentActionType string

const (
	IncidentNone   IncidentActionType = "none"
	IncidentUpsert IncidentActionType = "upsert"
	IncidentClose  IncidentActionType = "close"
)

type IncidentAction struct {
	Type              IncidentActionType
	Category          string
	Severity          string
	Summary           string
	RecommendedAction string
	StationID         string
}
