package app_test

import (
	"testing"

	"forge-mini/internal/app"
	"forge-mini/internal/orchestration"
)

func TestNewWithRuntimeInProcess(t *testing.T) {
	container, err := app.NewWithRuntime(orchestration.RuntimeInProcess)
	if err != nil {
		t.Fatal(err)
	}
	if container.Runtime == nil {
		t.Fatal("expected runtime to be configured")
	}
}

func TestNewWithRuntimeTemporalIsReserved(t *testing.T) {
	_, err := app.NewWithRuntime(orchestration.RuntimeTemporal)
	if err == nil {
		t.Fatal("expected temporal runtime to fail until it is wired")
	}
}

func TestNewFromEnvDefaultsToInProcess(t *testing.T) {
	t.Setenv("FORGE_RUNTIME", "")
	container, err := app.NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if container.Runtime == nil {
		t.Fatal("expected runtime to be configured")
	}
}
