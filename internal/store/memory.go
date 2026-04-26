package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"forge-mini/internal/incidents"
	"forge-mini/internal/projections"
	"forge-mini/internal/signals"
	"forge-mini/internal/workflow"
)

var ErrNotFound = errors.New("not found")

type Memory struct {
	mu sync.RWMutex

	signalsByID        map[string]signals.OperationalSignal
	signalsByUnit      map[string][]signals.OperationalSignal
	decisionsByWorkfow map[string][]workflow.WorkflowDecision
	workflowsByUnit    map[string]workflow.UnitWorkflow
	incidentsByID      map[string]incidents.Incident
	openIncidentByUnit map[string]string
	unitViews          map[string]projections.UnitStatusView
	incidentViews      map[string]projections.IncidentDashboardItem
	stationViews       map[string]projections.StationHealthView
	timersByKey        map[string]workflow.TimerPayload
}

func NewMemory() *Memory {
	return &Memory{
		signalsByID:        map[string]signals.OperationalSignal{},
		signalsByUnit:      map[string][]signals.OperationalSignal{},
		decisionsByWorkfow: map[string][]workflow.WorkflowDecision{},
		workflowsByUnit:    map[string]workflow.UnitWorkflow{},
		incidentsByID:      map[string]incidents.Incident{},
		openIncidentByUnit: map[string]string{},
		unitViews:          map[string]projections.UnitStatusView{},
		incidentViews:      map[string]projections.IncidentDashboardItem{},
		stationViews:       map[string]projections.StationHealthView{},
		timersByKey:        map[string]workflow.TimerPayload{},
	}
}

func (m *Memory) AppendSignal(ctx context.Context, signal signals.OperationalSignal) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signalsByID[signal.ID] = signal
	m.signalsByUnit[signal.UnitID] = append(m.signalsByUnit[signal.UnitID], signal)
	return nil
}

func (m *Memory) SignalExists(ctx context.Context, signalID string) (bool, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.signalsByID[signalID]
	return ok, nil
}

func (m *Memory) ListSignalsByUnit(ctx context.Context, unitID string) ([]signals.OperationalSignal, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := append([]signals.OperationalSignal(nil), m.signalsByUnit[unitID]...)
	return values, nil
}

func (m *Memory) AppendDecision(ctx context.Context, decision workflow.WorkflowDecision) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisionsByWorkfow[decision.WorkflowID] = append(m.decisionsByWorkfow[decision.WorkflowID], decision)
	return nil
}

func (m *Memory) ListDecisionsByWorkflow(ctx context.Context, workflowID string) ([]workflow.WorkflowDecision, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := append([]workflow.WorkflowDecision(nil), m.decisionsByWorkfow[workflowID]...)
	return values, nil
}

func (m *Memory) GetWorkflow(ctx context.Context, unitID string) (workflow.UnitWorkflow, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	wf, ok := m.workflowsByUnit[unitID]
	if !ok {
		return workflow.UnitWorkflow{}, ErrNotFound
	}
	return wf, nil
}

func (m *Memory) PutWorkflow(ctx context.Context, snapshot workflow.UnitWorkflow) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workflowsByUnit[snapshot.UnitID] = snapshot
	return nil
}

func (m *Memory) ListWorkflows(ctx context.Context) ([]workflow.UnitWorkflow, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]workflow.UnitWorkflow, 0, len(m.workflowsByUnit))
	for _, wf := range m.workflowsByUnit {
		values = append(values, wf)
	}
	return values, nil
}

func (m *Memory) GetOpenIncidentByUnit(ctx context.Context, unitID string) (*incidents.Incident, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.openIncidentByUnit[unitID]
	if !ok {
		return nil, nil
	}
	incident := m.incidentsByID[id]
	return &incident, nil
}

func (m *Memory) PutIncident(ctx context.Context, incident incidents.Incident) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidentsByID[incident.ID] = incident
	if incident.Status == incidents.StatusResolved {
		delete(m.openIncidentByUnit, incident.UnitID)
	} else {
		m.openIncidentByUnit[incident.UnitID] = incident.ID
	}
	return nil
}

func (m *Memory) GetIncidentByID(ctx context.Context, incidentID string) (*incidents.Incident, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	incident, ok := m.incidentsByID[incidentID]
	if !ok {
		return nil, ErrNotFound
	}
	return &incident, nil
}

func (m *Memory) ListOpenIncidents(ctx context.Context) ([]incidents.Incident, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := []incidents.Incident{}
	for _, incident := range m.incidentsByID {
		if incident.Status != incidents.StatusResolved {
			values = append(values, incident)
		}
	}
	return values, nil
}

func (m *Memory) PutUnitStatus(ctx context.Context, view projections.UnitStatusView) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unitViews[view.UnitID] = view
	return nil
}

func (m *Memory) GetUnitStatus(ctx context.Context, unitID string) (projections.UnitStatusView, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	view, ok := m.unitViews[unitID]
	if !ok {
		return projections.UnitStatusView{}, ErrNotFound
	}
	return view, nil
}

func (m *Memory) ListUnitsByState(ctx context.Context, state string) ([]projections.UnitStatusView, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := []projections.UnitStatusView{}
	for _, view := range m.unitViews {
		if state == "" || view.WorkflowState == state {
			values = append(values, view)
		}
	}
	return values, nil
}

func (m *Memory) PutIncidentView(ctx context.Context, view projections.IncidentDashboardItem) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidentViews[view.IncidentID] = view
	return nil
}

func (m *Memory) DeleteIncidentView(ctx context.Context, incidentID string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.incidentViews, incidentID)
	return nil
}

func (m *Memory) ListIncidentViews(ctx context.Context) ([]projections.IncidentDashboardItem, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]projections.IncidentDashboardItem, 0, len(m.incidentViews))
	for _, view := range m.incidentViews {
		values = append(values, view)
	}
	return values, nil
}

func (m *Memory) PutStationHealth(ctx context.Context, view projections.StationHealthView) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stationViews[view.StationID] = view
	return nil
}

func (m *Memory) ListStationHealth(ctx context.Context) ([]projections.StationHealthView, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]projections.StationHealthView, 0, len(m.stationViews))
	for _, view := range m.stationViews {
		values = append(values, view)
	}
	return values, nil
}

func (m *Memory) PutTimer(ctx context.Context, payload workflow.TimerPayload) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timersByKey[payload.Key] = payload
	return nil
}

func (m *Memory) DeleteTimer(ctx context.Context, key string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.timersByKey, key)
	return nil
}

func (m *Memory) GetTimer(ctx context.Context, key string) (workflow.TimerPayload, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	payload, ok := m.timersByKey[key]
	if !ok {
		return workflow.TimerPayload{}, ErrNotFound
	}
	return payload, nil
}

func (m *Memory) ListTimers(ctx context.Context) ([]workflow.TimerPayload, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]workflow.TimerPayload, 0, len(m.timersByKey))
	for _, payload := range m.timersByKey {
		values = append(values, payload)
	}
	return values, nil
}

func (m *Memory) Schedule(ctx context.Context, key string, fireAt time.Time, payload workflow.TimerPayload) error {
	payload.Key = key
	payload.FireAt = fireAt
	return m.PutTimer(ctx, payload)
}

func (m *Memory) Cancel(ctx context.Context, key string) error {
	return m.DeleteTimer(ctx, key)
}
