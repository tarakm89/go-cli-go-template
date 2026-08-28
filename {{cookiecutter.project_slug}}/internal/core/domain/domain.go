// Package domain holds the entities, value objects and rules that make up the
// core of the hexagon. It is the innermost layer: it imports nothing from this
// module and nothing from the outside world beyond the standard library.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Sentinel errors the core raises. Adapters translate transport failures into
// these so that the rest of the application never sees an HTTP or DNS error.
var (
	// ErrInvalidTarget means the target could not be understood at all.
	ErrInvalidTarget = errors.New("invalid target")
	// ErrUnreachable means the external system could not be contacted.
	ErrUnreachable = errors.New("target unreachable")
)

// State is the health of a target as judged by the core.
type State string

// The states a target can be in.
const (
	StateUp       State = "up"
	StateDegraded State = "degraded"
	StateDown     State = "down"
)

// String implements fmt.Stringer.
func (s State) String() string { return string(s) }

// Target is an external system the application knows how to talk to.
type Target struct {
	// Name is a short human readable label, derived from the host by default.
	Name string
	// Address is the absolute URL of the target's health endpoint.
	Address string
}

// NewTarget validates raw user input and turns it into a Target.
func NewTarget(raw string) (Target, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Target{}, fmt.Errorf("%w: empty address", ErrInvalidTarget)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return Target{}, fmt.Errorf("%w: %s: %w", ErrInvalidTarget, trimmed, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Target{}, fmt.Errorf("%w: %s: scheme must be http or https", ErrInvalidTarget, trimmed)
	}
	if parsed.Host == "" {
		return Target{}, fmt.Errorf("%w: %s: missing host", ErrInvalidTarget, trimmed)
	}

	return Target{Name: parsed.Host, Address: parsed.String()}, nil
}

// Probe is the raw observation an outbound adapter brings back from a target.
// It carries transport facts only; interpreting them is the core's job.
type Probe struct {
	StatusCode int
	Latency    time.Duration
}

// Health is the core's verdict about a target.
type Health struct {
	Target  Target
	State   State
	Latency time.Duration
	Detail  string
}

// Thresholds decide when a reachable target is considered merely degraded.
type Thresholds struct {
	// Degraded is the latency above which a healthy response is downgraded.
	Degraded time.Duration
}

// DefaultThresholds are used when the caller does not supply any.
func DefaultThresholds() Thresholds {
	return Thresholds{Degraded: 500 * time.Millisecond}
}

// Evaluate turns a raw probe into a verdict. This is the rule the whole
// application exists to apply, so it lives in the domain and is unit tested
// without any adapter, network or clock.
func (t Thresholds) Evaluate(target Target, probe Probe) Health {
	health := Health{Target: target, Latency: probe.Latency}

	switch {
	case probe.StatusCode >= 500:
		health.State = StateDown
		health.Detail = fmt.Sprintf("server error: status %d", probe.StatusCode)
	case probe.StatusCode >= 400:
		health.State = StateDegraded
		health.Detail = fmt.Sprintf("client error: status %d", probe.StatusCode)
	case probe.Latency > t.Degraded:
		health.State = StateDegraded
		health.Detail = fmt.Sprintf("slow response: %s over budget", (probe.Latency - t.Degraded).Round(time.Millisecond))
	default:
		health.State = StateUp
		health.Detail = fmt.Sprintf("status %d", probe.StatusCode)
	}

	return health
}

// Unreachable builds the verdict for a target that could not be contacted.
//
// The ErrUnreachable prefix is stripped from the detail: the state column
// already says the target is down, so repeating it would only push the useful
// part of the message off the edge of a terminal.
func Unreachable(target Target, cause error) Health {
	detail := "unreachable"
	if cause != nil {
		detail = strings.TrimPrefix(cause.Error(), ErrUnreachable.Error()+": ")
	}
	return Health{Target: target, State: StateDown, Detail: detail}
}

// Summary aggregates a set of verdicts into the worst state observed. An empty
// set is reported as up: there is nothing broken.
func Summary(checks []Health) State {
	worst := StateUp
	for _, check := range checks {
		switch check.State {
		case StateDown:
			return StateDown
		case StateDegraded:
			worst = StateDegraded
		case StateUp:
		}
	}
	return worst
}
