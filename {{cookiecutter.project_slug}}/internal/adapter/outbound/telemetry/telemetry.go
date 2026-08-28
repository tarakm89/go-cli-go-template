// Package telemetry instruments the ports.
//
// This is the piece that keeps observability out of the hexagon: rather than
// scattering spans and counters through the core, each decorator here wraps an
// existing port implementation and emits telemetry around it. The core cannot
// tell whether it is talking to a real adapter or an instrumented one, and the
// composition root decides.
package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/core/port"
)

// Attribute keys shared by the spans and metrics below.
const (
	attrTarget  = "target.name"
	attrAddress = "target.address"
	attrState   = "check.state"
	attrOutcome = "outcome"
)

// Instruments holds the metric instruments used by the decorators. Building
// them once and passing them around keeps the hot path free of lookups.
type Instruments struct {
	probeDuration metric.Float64Histogram
	probeTotal    metric.Int64Counter
	checkTotal    metric.Int64Counter
}

// NewInstruments registers this application's instruments on meter.
func NewInstruments(meter metric.Meter) (*Instruments, error) {
	probeDuration, err := meter.Float64Histogram(
		"probe.duration",
		metric.WithDescription("Time spent probing an external system."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	probeTotal, err := meter.Int64Counter(
		"probe.total",
		metric.WithDescription("Probes attempted, by outcome."),
		metric.WithUnit("{probe}"),
	)
	if err != nil {
		return nil, err
	}

	checkTotal, err := meter.Int64Counter(
		"check.total",
		metric.WithDescription("Health verdicts reached, by state."),
		metric.WithUnit("{check}"),
	)
	if err != nil {
		return nil, err
	}

	return &Instruments{
		probeDuration: probeDuration,
		probeTotal:    probeTotal,
		checkTotal:    checkTotal,
	}, nil
}

// InstrumentedProber wraps an outbound prober with a span, a duration
// histogram and an outcome counter.
type InstrumentedProber struct {
	next        port.Prober
	tracer      trace.Tracer
	instruments *Instruments
	logger      port.Logger
}

// NewProber decorates next. A nil tracer, instrument set or logger is
// tolerated, so this is safe to apply unconditionally.
func NewProber(next port.Prober, tracer trace.Tracer, instruments *Instruments, logger port.Logger) *InstrumentedProber {
	return &InstrumentedProber{next: next, tracer: tracer, instruments: instruments, logger: logger}
}

// Probe implements port.Prober.
func (p *InstrumentedProber) Probe(ctx context.Context, target domain.Target) (domain.Probe, error) {
	attrs := []attribute.KeyValue{
		attribute.String(attrTarget, target.Name),
		attribute.String(attrAddress, target.Address),
	}

	ctx, span := p.startSpan(ctx, "probe "+target.Name, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...))
	defer span.End()

	probe, err := p.next.Probe(ctx, target)

	outcome := "ok"
	if err != nil {
		outcome = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if errors.Is(err, domain.ErrUnreachable) {
			outcome = "unreachable"
		}
		p.log(ctx, "probe failed",
			"target", target.Name, "address", target.Address, "error", err.Error())
	} else {
		span.SetAttributes(
			attribute.Int("http.response.status_code", probe.StatusCode),
			attribute.Int64("probe.latency_ms", probe.Latency.Milliseconds()),
		)
		p.log(ctx, "probe completed",
			"target", target.Name, "status", probe.StatusCode, "latency_ms", probe.Latency.Milliseconds())
	}

	if p.instruments != nil {
		// A fresh slice: appending onto attrs would share its backing array.
		measured := make([]attribute.KeyValue, 0, len(attrs)+1)
		measured = append(measured, attrs...)
		measured = append(measured, attribute.String(attrOutcome, outcome))
		p.instruments.probeDuration.Record(ctx, probe.Latency.Seconds(),
			metric.WithAttributes(measured...))
		p.instruments.probeTotal.Add(ctx, 1, metric.WithAttributes(measured...))
	}

	return probe, err
}

// InstrumentedHealthChecker wraps the inbound port so that a whole run shows
// up as one span with a child per target.
type InstrumentedHealthChecker struct {
	next        port.HealthChecker
	tracer      trace.Tracer
	instruments *Instruments
}

// NewHealthChecker decorates next.
func NewHealthChecker(next port.HealthChecker, tracer trace.Tracer, instruments *Instruments) *InstrumentedHealthChecker {
	return &InstrumentedHealthChecker{next: next, tracer: tracer, instruments: instruments}
}

// Check implements port.HealthChecker.
func (h *InstrumentedHealthChecker) Check(ctx context.Context, target domain.Target) domain.Health {
	ctx, span := h.startSpan(ctx, "check "+target.Name,
		trace.WithAttributes(attribute.String(attrTarget, target.Name)))
	defer span.End()

	result := h.next.Check(ctx, target)
	h.record(ctx, span, result)

	return result
}

// CheckAll implements port.HealthChecker.
func (h *InstrumentedHealthChecker) CheckAll(ctx context.Context, targets []domain.Target) []domain.Health {
	ctx, span := h.startSpan(ctx, "check all",
		trace.WithAttributes(attribute.Int("check.target_count", len(targets))))
	defer span.End()

	results := h.next.CheckAll(ctx, targets)
	for _, result := range results {
		h.record(ctx, span, result)
	}
	span.SetAttributes(attribute.String("check.summary", domain.Summary(results).String()))

	return results
}

func (h *InstrumentedHealthChecker) record(ctx context.Context, span trace.Span, result domain.Health) {
	span.SetAttributes(attribute.String(attrState, result.State.String()))
	if result.State == domain.StateDown {
		span.SetStatus(codes.Error, result.Detail)
	}
	if h.instruments != nil {
		h.instruments.checkTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String(attrTarget, result.Target.Name),
			attribute.String(attrState, result.State.String()),
		))
	}
}

func (h *InstrumentedHealthChecker) startSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return startSpan(ctx, h.tracer, name, opts...)
}

func (p *InstrumentedProber) startSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return startSpan(ctx, p.tracer, name, opts...)
}

func (p *InstrumentedProber) log(ctx context.Context, msg string, attrs ...any) {
	if p.logger != nil {
		p.logger.Debug(ctx, msg, attrs...)
	}
}

// startSpan tolerates a nil tracer so the decorators can be applied even when
// telemetry was never set up.
func startSpan(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.Start(ctx, name, opts...)
}

var (
	_ port.Prober        = (*InstrumentedProber)(nil)
	_ port.HealthChecker = (*InstrumentedHealthChecker)(nil)
)
