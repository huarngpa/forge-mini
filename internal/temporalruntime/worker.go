package temporalruntime

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type WorkerHandle struct {
	worker worker.Worker
}

func StartWorker(temporalClient client.Client, taskQueue string) (*WorkerHandle, error) {
	if taskQueue == "" {
		taskQueue = DefaultTaskQueue
	}
	w := worker.New(temporalClient, taskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(UnitWorkflow, workflow.RegisterOptions{Name: UnitWorkflowName})
	if err := w.Start(); err != nil {
		return nil, err
	}
	return &WorkerHandle{worker: w}, nil
}

func (h *WorkerHandle) Stop() {
	if h != nil && h.worker != nil {
		h.worker.Stop()
	}
}
