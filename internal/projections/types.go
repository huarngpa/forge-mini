package projections

import "time"

type UnitStatusView struct {
	UnitID           string    `json:"unit_id"`
	WorkflowState    string    `json:"workflow_state"`
	CurrentStationID string    `json:"current_station_id"`
	ActiveHoldReason string    `json:"active_hold_reason"`
	OpenIncidentID   string    `json:"open_incident_id"`
	RetryCount       int       `json:"retry_count"`
	LastSignalType   string    `json:"last_signal_type"`
	LastDecisionType string    `json:"last_decision_type"`
	LastUpdatedAt    time.Time `json:"last_updated_at"`
}

type IncidentDashboardItem struct {
	IncidentID        string    `json:"incident_id"`
	UnitID            string    `json:"unit_id"`
	StationID         string    `json:"station_id"`
	Status            string    `json:"status"`
	Category          string    `json:"category"`
	Severity          string    `json:"severity"`
	RecommendedAction string    `json:"recommended_action"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type StationHealthView struct {
	StationID          string    `json:"station_id"`
	ActiveUnitID       string    `json:"active_unit_id"`
	LastHeartbeatAt    time.Time `json:"last_heartbeat_at"`
	CurrentStatus      string    `json:"current_status"`
	OpenIncidentCount  int       `json:"open_incident_count"`
	RetryCountLastHour int       `json:"retry_count_last_hour"`
}
