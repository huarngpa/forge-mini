package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"forge-mini/internal/app"
	"forge-mini/internal/signals"
	"forge-mini/internal/sim"
	"forge-mini/internal/store"
	"forge-mini/internal/workflow"
)

type Server struct {
	container *app.Container
}

func NewServer(container *app.Container) *Server {
	return &Server{container: container}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/signals", s.handleSignal)
	mux.HandleFunc("POST /api/timers", s.handleTimer)
	mux.HandleFunc("GET /api/units/", s.handleUnitRoutes)
	mux.HandleFunc("GET /api/dashboard/incidents", s.handleIncidents)
	mux.HandleFunc("GET /api/dashboard/stations", s.handleStations)
	mux.HandleFunc("GET /api/dashboard/units", s.handleUnitsByState)
	mux.HandleFunc("GET /api/triage/units/", s.handleTriageUnit)
	mux.HandleFunc("GET /api/timers", s.handleTimers)
	mux.HandleFunc("GET /api/demo/scenarios", s.handleScenarioList)
	mux.HandleFunc("POST /api/demo/scenarios/", s.handleScenarioRun)
	return mux
}

func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	var signal signals.OperationalSignal
	if err := json.NewDecoder(r.Body).Decode(&signal); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.container.Runtime.HandleSignal(r.Context(), signal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleTimer(w http.ResponseWriter, r *http.Request) {
	var timer workflow.TimerPayload
	if err := json.NewDecoder(r.Body).Decode(&timer); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.container.Runtime.HandleTimer(r.Context(), timer)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUnitRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/units/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "timeline" {
		s.handleTimeline(w, r, parts[0])
		return
	}
	if len(parts) == 1 {
		s.handleUnit(w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleUnit(w http.ResponseWriter, r *http.Request, unitID string) {
	wf, err := s.container.Memory.GetWorkflow(r.Context(), unitID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request, unitID string) {
	wf, err := s.container.Memory.GetWorkflow(r.Context(), unitID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	signalsList, _ := s.container.Memory.ListSignalsByUnit(r.Context(), unitID)
	decisions, _ := s.container.Memory.ListDecisionsByWorkflow(r.Context(), wf.WorkflowID)
	triage, _ := s.container.Triage.SummarizeUnit(r.Context(), unitID)
	writeJSON(w, http.StatusOK, map[string]any{
		"signals":               signalsList,
		"decisions":             decisions,
		"triage_recommendation": triage,
	})
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	items, err := s.container.Memory.ListIncidentViews(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleStations(w http.ResponseWriter, r *http.Request) {
	items, err := s.container.Memory.ListStationHealth(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUnitsByState(w http.ResponseWriter, r *http.Request) {
	items, err := s.container.Memory.ListUnitsByState(r.Context(), r.URL.Query().Get("state"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleTriageUnit(w http.ResponseWriter, r *http.Request) {
	unitID := strings.TrimPrefix(r.URL.Path, "/api/triage/units/")
	rec, err := s.container.Triage.SummarizeUnit(r.Context(), unitID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleTimers(w http.ResponseWriter, r *http.Request) {
	items, err := s.container.Memory.ListTimers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleScenarioList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"scenarios": sim.ScenarioNames(),
	})
}

func (s *Server) handleScenarioRun(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/demo/scenarios/")
	unitID := r.URL.Query().Get("unit_id")
	if unitID == "" {
		unitID = fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
	}
	result, err := s.container.Runtime.RunScenario(r.Context(), name, unitID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
