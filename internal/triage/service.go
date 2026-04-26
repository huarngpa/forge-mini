package triage

import (
	"context"
	"fmt"

	"forge-mini/internal/incidents"
	"forge-mini/internal/workflow"
)

type IncidentReader interface {
	GetOpenIncidentByUnit(ctx context.Context, unitID string) (*incidents.Incident, error)
}

type WorkflowReader interface {
	GetWorkflow(ctx context.Context, unitID string) (workflow.UnitWorkflow, error)
}

type Recommendation struct {
	Summary           string `json:"summary"`
	LikelyCause       string `json:"likely_cause"`
	RecommendedAction string `json:"recommended_action"`
}

type Service struct {
	incidents IncidentReader
	workflows WorkflowReader
}

func New(incidents IncidentReader, workflows WorkflowReader) *Service {
	return &Service{incidents: incidents, workflows: workflows}
}

func (s *Service) SummarizeUnit(ctx context.Context, unitID string) (Recommendation, error) {
	incident, err := s.incidents.GetOpenIncidentByUnit(ctx, unitID)
	if err != nil {
		return Recommendation{}, err
	}
	if incident == nil {
		wf, err := s.workflows.GetWorkflow(ctx, unitID)
		if err != nil {
			return Recommendation{}, err
		}
		return Recommendation{
			Summary:           fmt.Sprintf("unit %s is currently in state %s", unitID, wf.State),
			LikelyCause:       "no active incident",
			RecommendedAction: "continue workflow",
		}, nil
	}
	return summarizeIncident(*incident), nil
}

func summarizeIncident(incident incidents.Incident) Recommendation {
	switch incident.Category {
	case "test_flake":
		return Recommendation{
			Summary:           "the unit saw a flaky test infrastructure failure and entered retry flow",
			LikelyCause:       "fixture or communications instability",
			RecommendedAction: "retry_test",
		}
	case "repeated_failure":
		return Recommendation{
			Summary:           "the unit failed repeatedly and was placed on hold",
			LikelyCause:       "repeat failure after retry budget exhausted",
			RecommendedAction: "route_to_rework",
		}
	case "heartbeat_timeout":
		return Recommendation{
			Summary:           "the active station stopped sending expected signals",
			LikelyCause:       "station outage or stale workflow state",
			RecommendedAction: "investigate_station",
		}
	default:
		return Recommendation{
			Summary:           incident.Summary,
			LikelyCause:       incident.Category,
			RecommendedAction: incident.RecommendedAction,
		}
	}
}
