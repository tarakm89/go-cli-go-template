package observability

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"go.opentelemetry.io/otel/trace"
)

// consoleHandler builds the stderr handler. JSON is the sensible default in a
// pipeline, text is nicer at a terminal; both are offered because the same
// binary gets used in both places.
func consoleHandler(cfg Config) slog.Handler {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == LogFormatJSON {
		return slog.NewJSONHandler(cfg.Stderr, opts)
	}
	return slog.NewTextHandler(cfg.Stderr, opts)
}

// fanoutHandler writes every record to each of its handlers, and stamps the
// active trace and span id onto it. Correlating a CI log line with the trace it
// came from is the whole point of running the log pipeline at all.
type fanoutHandler struct {
	handlers []slog.Handler
}

func newFanoutHandler(handlers ...slog.Handler) slog.Handler {
	return &fanoutHandler{handlers: handlers}
}

// Enabled reports whether any underlying handler wants the record.
func (f *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return slices.ContainsFunc(f.handlers, func(h slog.Handler) bool {
		return h.Enabled(ctx, level)
	})
}

// Handle stamps trace correlation onto the record and forwards it.
func (f *fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.TraceID().String()),
			slog.String("span_id", span.SpanID().String()),
		)
	}

	var errs []error
	for _, handler := range f.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		// Each handler gets its own copy: Handle may retain the record.
		if err := handler.Handle(ctx, record.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WithAttrs implements slog.Handler.
func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, handler := range f.handlers {
		next[i] = handler.WithAttrs(slices.Clone(attrs))
	}
	return &fanoutHandler{handlers: next}
}

// WithGroup implements slog.Handler.
func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return f
	}
	next := make([]slog.Handler, len(f.handlers))
	for i, handler := range f.handlers {
		next[i] = handler.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}
