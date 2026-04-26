package policy

import "time"

type RetryPolicyResult struct {
	RetryAuthorized bool
	MaxAttempts     int
	Backoff         time.Duration
	ReasonCode      string
}

type RetryPolicy interface {
	Evaluate(failureClass string, attempt int) RetryPolicyResult
}

type DefaultRetryPolicy struct{}

func (DefaultRetryPolicy) Evaluate(failureClass string, attempt int) RetryPolicyResult {
	switch failureClass {
	case "flaky_infra":
		if attempt < 1 {
			return RetryPolicyResult{
				RetryAuthorized: true,
				MaxAttempts:     1,
				Backoff:         2 * time.Second,
				ReasonCode:      "retryable_failure",
			}
		}
		return RetryPolicyResult{
			RetryAuthorized: false,
			MaxAttempts:     1,
			ReasonCode:      "retry_budget_exhausted",
		}
	default:
		return RetryPolicyResult{
			RetryAuthorized: false,
			MaxAttempts:     0,
			ReasonCode:      "retry_not_allowed",
		}
	}
}
