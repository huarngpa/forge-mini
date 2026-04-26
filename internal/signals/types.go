package signals

import (
	"encoding/json"
	"time"
)

type SignalType string

const (
	SignalUnitAdmitted        SignalType = "unit.admitted"
	SignalStationEntered      SignalType = "station.entered"
	SignalAssemblyCompleted   SignalType = "assembly.completed"
	SignalTestStarted         SignalType = "test.started"
	SignalTestPassed          SignalType = "test.passed"
	SignalTestFailed          SignalType = "test.failed"
	SignalHeartbeatReceived   SignalType = "station.heartbeat"
	SignalQualityDisposition  SignalType = "quality.disposition"
	SignalReworkStarted       SignalType = "rework.started"
	SignalReworkCompleted     SignalType = "rework.completed"
	SignalInspectionCompleted SignalType = "inspection.completed"
	SignalOperatorOverride    SignalType = "operator.override"
)

type OperationalSignal struct {
	ID            string          `json:"id"`
	Type          SignalType      `json:"type"`
	UnitID        string          `json:"unit_id"`
	Source        string          `json:"source"`
	StationID     string          `json:"station_id"`
	CorrelationID string          `json:"correlation_id"`
	Sequence      int64           `json:"sequence"`
	OccurredAt    time.Time       `json:"occurred_at"`
	ReceivedAt    time.Time       `json:"received_at"`
	Payload       json.RawMessage `json:"payload"`
}

type TestFailedPayload struct {
	FailureCode        string             `json:"failure_code"`
	FailureClass       string             `json:"failure_class"`
	Message            string             `json:"message"`
	MeasurementSummary map[string]float64 `json:"measurement_summary"`
}

type QualityDispositionPayload struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
	Actor  string `json:"actor"`
}

type OperatorOverridePayload struct {
	Action string `json:"action"`
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}
