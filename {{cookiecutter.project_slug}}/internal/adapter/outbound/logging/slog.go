// Package logging adapts log/slog to the port.Logger outbound port.
//
// The logger it wraps is the one built by internal/observability, so every
// record reaches both the console and the OTLP log pipeline, carrying the
// trace and span id of whatever is running.
package logging

import (
	"context"
	"log/slog"

	"{{ cookiecutter.module_path }}/internal/core/port"
)

// Adapter implements port.Logger on top of a *slog.Logger.
type Adapter struct{ logger *slog.Logger }

// New wraps an slog logger. A nil logger yields a no-op adapter, so callers
// never have to guard against one.
func New(logger *slog.Logger) *Adapter {
	if logger == nil {
		return &Adapter{logger: slog.New(discardHandler{})}
	}
	return &Adapter{logger: logger}
}

// Nop returns a logger that discards everything.
func Nop() *Adapter { return New(nil) }

// Debug implements port.Logger.
func (a *Adapter) Debug(ctx context.Context, msg string, attrs ...any) {
	a.logger.DebugContext(ctx, msg, attrs...)
}

// Info implements port.Logger.
func (a *Adapter) Info(ctx context.Context, msg string, attrs ...any) {
	a.logger.InfoContext(ctx, msg, attrs...)
}

// Warn implements port.Logger.
func (a *Adapter) Warn(ctx context.Context, msg string, attrs ...any) {
	a.logger.WarnContext(ctx, msg, attrs...)
}

// Error implements port.Logger.
func (a *Adapter) Error(ctx context.Context, msg string, attrs ...any) {
	a.logger.ErrorContext(ctx, msg, attrs...)
}

// discardHandler drops every record. slog.DiscardHandler exists from Go 1.24,
// but spelling it out keeps this adapter readable.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }

var _ port.Logger = (*Adapter)(nil)
