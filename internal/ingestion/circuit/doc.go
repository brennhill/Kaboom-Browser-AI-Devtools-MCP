// Purpose: Package circuit — capture ingest circuit breaker and rate-limiting state machine.
// Why: Protects daemon stability by throttling abusive event rates and exposing health state.
// Docs: docs/features/feature/rate-limiting/index.md

/*
Package circuit implements a circuit breaker for the capture ingest path.

When event rates exceed 1000/second for 5 consecutive seconds, the circuit opens
and rejects new events until rates fall below threshold for 10 seconds.

Key types:
  - Breaker: state machine tracking event rates with open/closed/half-open transitions.
  - HealthResponse: JSON-serializable health status returned by the /health endpoint.

Key functions:
  - NewBreaker: creates a breaker with default thresholds.
  - Allow: checks whether an incoming event should be accepted or rejected.
*/
package circuit
