// Package port declares the boundary of the hexagon.
//
// Inbound (driving) ports are the use cases the application offers; the CLI
// adapter calls them. Outbound (driven) ports are what the application needs
// from the outside world; adapters implement them.
//
// Every signature here is expressed in domain types and the standard library
// only. No adapter package may appear in this file — that is what lets the
// functional tests swap real adapters for fakes without touching the core.
package port

import (
	"context"
	"time"

	"{{ cookiecutter.module_path }}/internal/core/domain"
)

// HealthChecker is the inbound port: the use case the CLI drives.
type HealthChecker interface {
	// Check inspects a single target. Failure is part of the result, not an
	// error: an unreachable target is a legitimate verdict.
	Check(ctx context.Context, target domain.Target) domain.Health
	// CheckAll inspects every target concurrently, preserving input order.
	CheckAll(ctx context.Context, targets []domain.Target) []domain.Health
}

// Prober is an outbound port: it reaches an external system and reports what
// it saw. Implementations must map every transport failure to an error that
// wraps domain.ErrUnreachable, so the core never sees an HTTP or DNS error.
type Prober interface {
	Probe(ctx context.Context, target domain.Target) (domain.Probe, error)
}

// Reporter is an outbound port for presenting results. Rendering is a detail,
// so it sits behind a port like any other external concern.
type Reporter interface {
	Report(checks []domain.Health) error
}

// Clock is an outbound port over the passage of time, so that latency is
// deterministic under test.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// Logger is an outbound port over structured logging. The core logs through
// this interface rather than through log/slog directly, so that a test can
// assert on what was logged and so that the choice of backend stays in the
// composition root.
//
// Attributes are alternating key/value pairs, as in log/slog.
type Logger interface {
	Debug(ctx context.Context, msg string, attrs ...any)
	Info(ctx context.Context, msg string, attrs ...any)
	Warn(ctx context.Context, msg string, attrs ...any)
	Error(ctx context.Context, msg string, attrs ...any)
}
