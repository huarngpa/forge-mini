package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"forge-mini/internal/policy"
	"forge-mini/internal/signals"
)

type TransitionEngine interface {
	ApplySignal(workflow UnitWorkflow, signal signals.OperationalSignal, now time.Time) (TransitionResult, error)
	ApplyTimer(workflow UnitWorkflow, timer TimerPayload, now time.Time) (TransitionResult, error)
}

type Engine struct {
	policy policy.RetryPolicy
}

func NewEngine(retryPolicy policy.RetryPolicy) *Engine {
	return &Engine{policy: retryPolicy}
}

func (e *Engine) ApplySignal(wf UnitWorkflow, signal signals.OperationalSignal, now time.Time) (TransitionResult, error) {
	if wf.WorkflowID == "" {
		wf = New(signal.UnitID, now)
	}
	result := TransitionResult{NextWorkflow: wf}
	result.Decisions = append(result.Decisions, newDecision(wf, DecisionAcceptSignal, "valid_transition", string(signal.Type), signal.Source, now))

	next := result.NextWorkflow
	next.LastSignalID = signal.ID
	next.LastSignalType = string(signal.Type)
	next.LastSignalAt = now
	next.UpdatedAt = now

	switch signal.Type {
	case signals.SignalUnitAdmitted:
		if next.State != StatePending {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		result.Decisions = append(result.Decisions, transitionDecision(next, StateReadyForAssembly, now))
		next.State = StateReadyForAssembly

	case signals.SignalStationEntered:
		switch next.State {
		case StateReadyForAssembly:
			next.State = StateInAssembly
			next.CurrentStationID = signal.StationID
			result.Decisions = append(result.Decisions, transitionDecision(next, StateInAssembly, now))
			result.TimersToStart = append(result.TimersToStart, TimerRequest{
				Key:       timerKey(signal.UnitID, TimerHeartbeatTimeout, signal.StationID),
				TimerType: TimerHeartbeatTimeout,
				StationID: signal.StationID,
				FireAt:    now.Add(30 * time.Second),
			})
		default:
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}

	case signals.SignalAssemblyCompleted:
		if next.State != StateInAssembly {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		next.State = StateReadyForTest
		next.CurrentStationID = "test"
		result.Decisions = append(result.Decisions,
			newDecision(next, DecisionCancelTimer, "assembly_completed", TimerHeartbeatTimeout, signal.Source, now),
			transitionDecision(next, StateReadyForTest, now),
		)
		result.TimersToCancel = append(result.TimersToCancel, timerKey(signal.UnitID, TimerHeartbeatTimeout, signal.StationID))

	case signals.SignalTestStarted:
		if next.State != StateReadyForTest {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		next.State = StateAwaitingTestResult
		next.CurrentStationID = "test"
		result.Decisions = append(result.Decisions, transitionDecision(next, StateAwaitingTestResult, now))
		result.TimersToStart = append(result.TimersToStart,
			TimerRequest{
				Key:       timerKey(signal.UnitID, TimerTestResultTimeout, "test"),
				TimerType: TimerTestResultTimeout,
				StationID: "test",
				FireAt:    now.Add(45 * time.Second),
			},
			TimerRequest{
				Key:       timerKey(signal.UnitID, TimerHeartbeatTimeout, "test"),
				TimerType: TimerHeartbeatTimeout,
				StationID: "test",
				FireAt:    now.Add(30 * time.Second),
			},
		)

	case signals.SignalTestFailed:
		if next.State != StateAwaitingTestResult {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		var payload signals.TestFailedPayload
		if err := json.Unmarshal(signal.Payload, &payload); err != nil {
			return TransitionResult{}, fmt.Errorf("decode test failure payload: %w", err)
		}
		attempt := next.AttemptCountByStation["test"]
		policyResult := e.policy.Evaluate(payload.FailureClass, attempt)
		next.AttemptCountByStation["test"] = attempt + 1
		result.TimersToCancel = append(result.TimersToCancel, timerKey(signal.UnitID, TimerTestResultTimeout, "test"))
		result.Decisions = append(result.Decisions, newDecision(next, DecisionCancelTimer, "test_result_recorded", TimerTestResultTimeout, signal.Source, now))
		if policyResult.RetryAuthorized {
			next.State = StateRetryPending
			result.Decisions = append(result.Decisions,
				newDecision(next, DecisionStartRetry, policyResult.ReasonCode, payload.FailureCode, signal.Source, now),
				transitionDecision(next, StateRetryPending, now),
			)
			result.IncidentAction = IncidentAction{
				Type:              IncidentUpsert,
				Category:          "test_flake",
				Severity:          "medium",
				Summary:           payload.Message,
				RecommendedAction: "retry_test",
				StationID:         "test",
			}
			result.TimersToStart = append(result.TimersToStart, TimerRequest{
				Key:       timerKey(signal.UnitID, TimerRetryBackoff, "test"),
				TimerType: TimerRetryBackoff,
				StationID: "test",
				FireAt:    now.Add(policyResult.Backoff),
			})
		} else {
			next.State = StateOnHold
			next.ActiveHoldReason = policyResult.ReasonCode
			result.Decisions = append(result.Decisions,
				newDecision(next, DecisionPlaceHold, policyResult.ReasonCode, payload.FailureCode, signal.Source, now),
				newDecision(next, DecisionEscalate, policyResult.ReasonCode, "manual review required", signal.Source, now),
				transitionDecision(next, StateOnHold, now),
			)
			result.IncidentAction = IncidentAction{
				Type:              IncidentUpsert,
				Category:          "repeated_failure",
				Severity:          "high",
				Summary:           payload.Message,
				RecommendedAction: "route_to_rework",
				StationID:         "test",
			}
			result.TimersToStart = append(result.TimersToStart, TimerRequest{
				Key:       timerKey(signal.UnitID, TimerManualReviewSLA, "test"),
				TimerType: TimerManualReviewSLA,
				StationID: "test",
				FireAt:    now.Add(15 * time.Minute),
			})
		}

	case signals.SignalTestPassed:
		if next.State != StateAwaitingTestResult {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		next.State = StateReadyForInspection
		next.ActiveHoldReason = ""
		result.Decisions = append(result.Decisions,
			newDecision(next, DecisionCancelTimer, "test_result_recorded", TimerTestResultTimeout, signal.Source, now),
			transitionDecision(next, StateReadyForInspection, now),
			newDecision(next, DecisionCompleteIncident, "test_passed", "incident resolved", signal.Source, now),
		)
		result.TimersToCancel = append(result.TimersToCancel, timerKey(signal.UnitID, TimerTestResultTimeout, "test"))
		result.IncidentAction = IncidentAction{
			Type: IncidentClose,
		}

	case signals.SignalHeartbeatReceived:
		if !slices.Contains([]string{StateInAssembly, StateAwaitingTestResult, StateInRework, StateInInspection}, next.State) {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		result.Decisions = append(result.Decisions,
			newDecision(next, DecisionScheduleTimer, "heartbeat_refresh", TimerHeartbeatTimeout, signal.Source, now),
		)
		result.TimersToStart = append(result.TimersToStart, TimerRequest{
			Key:       timerKey(signal.UnitID, TimerHeartbeatTimeout, signal.StationID),
			TimerType: TimerHeartbeatTimeout,
			StationID: signal.StationID,
			FireAt:    now.Add(30 * time.Second),
		})

	case signals.SignalQualityDisposition:
		if next.State != StateOnHold {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		var payload signals.QualityDispositionPayload
		if err := json.Unmarshal(signal.Payload, &payload); err != nil {
			return TransitionResult{}, fmt.Errorf("decode quality disposition payload: %w", err)
		}
		switch payload.Action {
		case "route_to_rework":
			next.State = StateAwaitingRework
			next.ActiveHoldReason = ""
			result.Decisions = append(result.Decisions,
				newDecision(next, DecisionRouteToRework, "rework_approved", payload.Reason, payload.Actor, now),
				transitionDecision(next, StateAwaitingRework, now),
			)
		case "scrap":
			next.State = StateScrapped
			next.ActiveHoldReason = ""
			result.Decisions = append(result.Decisions, transitionDecision(next, StateScrapped, now))
		default:
			return reject(wf, signal, now, "unsupported_quality_action"), nil
		}

	case signals.SignalReworkStarted:
		if next.State != StateAwaitingRework {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		next.State = StateInRework
		next.CurrentStationID = "rework"
		result.Decisions = append(result.Decisions, transitionDecision(next, StateInRework, now))

	case signals.SignalReworkCompleted:
		if next.State != StateInRework {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		next.State = StateReadyForTest
		next.CurrentStationID = "test"
		result.Decisions = append(result.Decisions, transitionDecision(next, StateReadyForTest, now))

	case signals.SignalInspectionCompleted:
		if next.State != StateInInspection && next.State != StateReadyForInspection {
			return reject(wf, signal, now, "unexpected_signal_for_state"), nil
		}
		next.State = StateCompleted
		result.Decisions = append(result.Decisions, transitionDecision(next, StateCompleted, now))

	default:
		return reject(wf, signal, now, "unsupported_signal_type"), nil
	}

	for _, req := range result.TimersToStart {
		if !slices.Contains(next.ActiveTimerKeys, req.Key) {
			next.ActiveTimerKeys = append(next.ActiveTimerKeys, req.Key)
		}
	}
	for _, key := range result.TimersToCancel {
		next.ActiveTimerKeys = removeTimer(next.ActiveTimerKeys, key)
	}

	next.LastDecisionAt = now
	result.NextWorkflow = next
	return result, nil
}

func (e *Engine) ApplyTimer(wf UnitWorkflow, timer TimerPayload, now time.Time) (TransitionResult, error) {
	result := TransitionResult{NextWorkflow: wf}
	switch timer.TimerType {
	case TimerRetryBackoff:
		if wf.State != StateRetryPending {
			result.Decisions = append(result.Decisions, newDecision(wf, DecisionRejectSignal, "unexpected_timer_for_state", timer.Key, "timer", now))
			return result, nil
		}
		next := wf
		next.State = StateAwaitingTestResult
		next.ActiveTimerKeys = removeTimer(next.ActiveTimerKeys, timer.Key)
		result.Decisions = append(result.Decisions,
			newDecision(wf, DecisionTransitionState, "retry_window_open", StateAwaitingTestResult, "timer", now),
			newDecision(wf, DecisionScheduleTimer, "await_test_result_after_retry", TimerTestResultTimeout, "timer", now),
		)
		result.TimersToStart = append(result.TimersToStart, TimerRequest{
			Key:       timerKey(wf.UnitID, TimerTestResultTimeout, timer.StationID),
			TimerType: TimerTestResultTimeout,
			StationID: timer.StationID,
			FireAt:    now.Add(45 * time.Second),
		})
		next.ActiveTimerKeys = append(next.ActiveTimerKeys, timerKey(wf.UnitID, TimerTestResultTimeout, timer.StationID))
		next.LastDecisionAt = now
		next.UpdatedAt = now
		result.NextWorkflow = next
		return result, nil

	case TimerHeartbeatTimeout, TimerTestResultTimeout:
		next := wf
		next.State = StateOnHold
		next.ActiveHoldReason = timer.TimerType
		next.ActiveTimerKeys = removeTimer(next.ActiveTimerKeys, timer.Key)
		result.Decisions = append(result.Decisions,
			newDecision(wf, DecisionPlaceHold, timer.TimerType, "timer fired", "timer", now),
			newDecision(wf, DecisionEscalate, timer.TimerType, "manual investigation required", "timer", now),
			transitionDecision(wf, StateOnHold, now),
		)
		result.IncidentAction = IncidentAction{
			Type:              IncidentUpsert,
			Category:          "heartbeat_timeout",
			Severity:          "high",
			Summary:           "workflow timer fired without expected external signal",
			RecommendedAction: "investigate_station",
			StationID:         timer.StationID,
		}
		result.NextWorkflow = next
		return result, nil
	default:
		result.Decisions = append(result.Decisions, newDecision(wf, DecisionRejectSignal, "unsupported_timer_type", timer.TimerType, "timer", now))
		return result, nil
	}
}

func reject(wf UnitWorkflow, signal signals.OperationalSignal, now time.Time, reason string) TransitionResult {
	if wf.WorkflowID == "" {
		wf = New(signal.UnitID, now)
	}
	return TransitionResult{
		NextWorkflow: wf,
		Decisions: []WorkflowDecision{
			newDecision(wf, DecisionRejectSignal, reason, string(signal.Type), signal.Source, now),
		},
	}
}

func newDecision(wf UnitWorkflow, kind DecisionType, reason, summary, triggeredBy string, now time.Time) WorkflowDecision {
	return WorkflowDecision{
		ID:           fmt.Sprintf("dec-%d-%s", now.UnixNano(), kind),
		WorkflowID:   wf.WorkflowID,
		UnitID:       wf.UnitID,
		DecisionType: kind,
		ReasonCode:   reason,
		Summary:      summary,
		TriggeredBy:  triggeredBy,
		At:           now,
		Metadata:     map[string]any{},
	}
}

func transitionDecision(wf UnitWorkflow, nextState string, now time.Time) WorkflowDecision {
	return WorkflowDecision{
		ID:           fmt.Sprintf("dec-%d-transition", now.UnixNano()),
		WorkflowID:   wf.WorkflowID,
		UnitID:       wf.UnitID,
		DecisionType: DecisionTransitionState,
		ReasonCode:   "valid_transition",
		Summary:      nextState,
		TriggeredBy:  "orchestrator",
		At:           now,
		Metadata:     map[string]any{"next_state": nextState},
	}
}

func timerKey(unitID, timerType, stationID string) string {
	return fmt.Sprintf("%s:%s:%s", unitID, timerType, stationID)
}

func removeTimer(values []string, target string) []string {
	out := values[:0]
	for _, v := range values {
		if v != target {
			out = append(out, v)
		}
	}
	return out
}

type Orchestrator interface {
	HandleSignal(ctx context.Context, signal signals.OperationalSignal) (HandleResult, error)
	HandleTimer(ctx context.Context, timer TimerPayload) (HandleResult, error)
}
