package incidents

import "time"

type Status string

const (
	StatusOpen      Status = "open"
	StatusEscalated Status = "escalated"
	StatusResolved  Status = "resolved"
)

type Incident struct {
	ID                string    `json:"id"`
	UnitID            string    `json:"unit_id"`
	WorkflowID        string    `json:"workflow_id"`
	StationID         string    `json:"station_id"`
	Status            Status    `json:"status"`
	Category          string    `json:"category"`
	Severity          string    `json:"severity"`
	OpenedAt          time.Time `json:"opened_at"`
	LastUpdatedAt     time.Time `json:"last_updated_at"`
	RecommendedAction string    `json:"recommended_action"`
	AssignedTo        string    `json:"assigned_to"`
	Summary           string    `json:"summary"`
}

