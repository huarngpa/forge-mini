package temporalruntime

import (
	"forge-mini/internal/policy"
	"forge-mini/internal/signals"
	domainworkflow "forge-mini/internal/workflow"
	temporalsdk "go.temporal.io/sdk/workflow"
)

type scheduledTimer struct {
	payload domainworkflow.TimerPayload
	cancel  temporalsdk.CancelFunc
	future  temporalsdk.Future
}

func UnitWorkflow(ctx temporalsdk.Context, input UnitWorkflowInput) (domainworkflow.UnitWorkflow, error) {
	engine := domainworkflow.NewEngine(policy.DefaultRetryPolicy{})
	wf := domainworkflow.New(input.UnitID, temporalsdk.Now(ctx))
	timers := map[string]scheduledTimer{}

	if err := temporalsdk.SetQueryHandler(ctx, QueryWorkflowState, func() (domainworkflow.UnitWorkflow, error) {
		return wf, nil
	}); err != nil {
		return wf, err
	}

	signalCh := temporalsdk.GetSignalChannel(ctx, SignalOperational)
	timerCh := temporalsdk.GetSignalChannel(ctx, SignalManualTimer)

	for !isTerminal(wf.State) {
		selector := temporalsdk.NewSelector(ctx)
		selector.AddReceive(signalCh, func(c temporalsdk.ReceiveChannel, more bool) {
			var signal signals.OperationalSignal
			c.Receive(ctx, &signal)
			result, err := engine.ApplySignal(wf, signal, temporalsdk.Now(ctx))
			if err != nil {
				temporalsdk.GetLogger(ctx).Error("apply signal failed", "error", err, "signal_id", signal.ID)
				return
			}
			wf = result.NextWorkflow
			applyTimerChanges(ctx, wf, timers, result)
		})
		selector.AddReceive(timerCh, func(c temporalsdk.ReceiveChannel, more bool) {
			var timer domainworkflow.TimerPayload
			c.Receive(ctx, &timer)
			result, err := engine.ApplyTimer(wf, timer, temporalsdk.Now(ctx))
			if err != nil {
				temporalsdk.GetLogger(ctx).Error("apply manual timer failed", "error", err, "timer_key", timer.Key)
				return
			}
			wf = result.NextWorkflow
			applyTimerChanges(ctx, wf, timers, result)
		})
		for key, scheduled := range timers {
			timerKey := key
			timerPayload := scheduled.payload
			selector.AddFuture(scheduled.future, func(f temporalsdk.Future) {
				delete(timers, timerKey)
				if err := f.Get(ctx, nil); err != nil {
					return
				}
				result, err := engine.ApplyTimer(wf, timerPayload, temporalsdk.Now(ctx))
				if err != nil {
					temporalsdk.GetLogger(ctx).Error("apply temporal timer failed", "error", err, "timer_key", timerKey)
					return
				}
				wf = result.NextWorkflow
				applyTimerChanges(ctx, wf, timers, result)
			})
		}
		selector.Select(ctx)
	}

	for _, scheduled := range timers {
		scheduled.cancel()
	}
	return wf, nil
}

func applyTimerChanges(ctx temporalsdk.Context, wf domainworkflow.UnitWorkflow, timers map[string]scheduledTimer, result domainworkflow.TransitionResult) {
	for _, key := range result.TimersToCancel {
		if scheduled, ok := timers[key]; ok {
			scheduled.cancel()
			delete(timers, key)
		}
	}

	for _, req := range result.TimersToStart {
		if scheduled, ok := timers[req.Key]; ok {
			scheduled.cancel()
		}
		timerCtx, cancel := temporalsdk.WithCancel(ctx)
		duration := req.FireAt.Sub(temporalsdk.Now(ctx))
		if duration < 0 {
			duration = 0
		}
		timers[req.Key] = scheduledTimer{
			payload: domainworkflow.TimerPayload{
				Key:        req.Key,
				WorkflowID: wf.WorkflowID,
				UnitID:     wf.UnitID,
				TimerType:  req.TimerType,
				StationID:  req.StationID,
				FireAt:     temporalsdk.Now(ctx).Add(duration),
			},
			cancel: cancel,
			future: temporalsdk.NewTimer(timerCtx, duration),
		}
	}
}

func isTerminal(state string) bool {
	return state == domainworkflow.StateCompleted || state == domainworkflow.StateScrapped
}
