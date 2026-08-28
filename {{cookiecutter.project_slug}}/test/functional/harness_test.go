package functional_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"{{ cookiecutter.module_path }}/internal/adapter/inbound/cli"
	"{{ cookiecutter.module_path }}/internal/adapter/outbound/fake"
	"{{ cookiecutter.module_path }}/internal/bootstrap"
	"{{ cookiecutter.module_path }}/internal/buildinfo"
	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/observability"
)

// harness is one fully wired application with fake outbound adapters and
// in-memory telemetry. Build one per spec so nothing leaks between them.
type harness struct {
	Prober *fake.Prober

	stdout bytes.Buffer
	stderr bytes.Buffer

	spans  *tracetest.InMemoryExporter
	reader *sdkmetric.ManualReader

	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	telemetry      *observability.Telemetry
}

func newHarness() *harness {
	h := &harness{
		Prober: fake.NewProber(),
		spans:  tracetest.NewInMemoryExporter(),
		reader: sdkmetric.NewManualReader(),
	}

	// Synchronous export keeps the assertions free of eventual consistency.
	h.tracerProvider = sdktrace.NewTracerProvider(sdktrace.WithSyncer(h.spans))
	h.meterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(h.reader))
	h.telemetry = observability.WithProviders(
		"functional",
		h.tracerProvider,
		h.meterProvider,
		slog.New(slog.NewTextHandler(&h.stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)

	return h
}

// Run executes the CLI exactly as the shipped binary would and returns the
// process exit code.
func (h *harness) Run(args ...string) int {
	return bootstrap.Run(context.Background(), bootstrap.Options{
		Args:   args,
		Stdout: &h.stdout,
		Stderr: &h.stderr,
		Adapters: cli.Options{
			Build:     buildinfo.Info{Name: "testcli", Version: "0.0.0-test"},
			Prober:    h.Prober,
			Telemetry: h.telemetry,
		},
	})
}

// Stdout returns everything the command printed for the user.
func (h *harness) Stdout() string { return h.stdout.String() }

// Stderr returns logs and error messages.
func (h *harness) Stderr() string { return h.stderr.String() }

// SpanNames returns the names of the spans the run produced, in the order
// they ended.
func (h *harness) SpanNames() []string {
	spans := h.spans.GetSpans()
	names := make([]string, 0, len(spans))
	for i := range spans {
		names = append(names, spans[i].Name)
	}
	return names
}

// SpanNamed returns the span with the given name, and whether it was found.
func (h *harness) SpanNamed(name string) (tracetest.SpanStub, bool) {
	spans := h.spans.GetSpans()
	for i := range spans {
		if spans[i].Name == name {
			return spans[i], true
		}
	}
	return tracetest.SpanStub{}, false
}

// ParentOf returns the name of the span that the named span hangs off, which
// is how a test asserts on the shape of a trace rather than just its contents.
func (h *harness) ParentOf(name string) string {
	child, ok := h.SpanNamed(name)
	if !ok {
		return ""
	}
	spans := h.spans.GetSpans()
	for i := range spans {
		if spans[i].SpanContext.SpanID() == child.Parent.SpanID() {
			return spans[i].Name
		}
	}
	return ""
}

// MetricNames returns the names of the instruments that recorded something.
func (h *harness) MetricNames() []string {
	var collected metricdata.ResourceMetrics
	if err := h.reader.Collect(context.Background(), &collected); err != nil {
		return nil
	}

	names := make([]string, 0)
	for _, scope := range collected.ScopeMetrics {
		for _, metric := range scope.Metrics {
			names = append(names, metric.Name)
		}
	}
	return names
}

// Close releases the telemetry providers created for this harness.
func (h *harness) Close() {
	ctx := context.Background()
	_ = h.tracerProvider.Shutdown(ctx)
	_ = h.meterProvider.Shutdown(ctx)
}

// unreachable builds the error a real outbound adapter would return when it
// cannot reach an external system.
func unreachable(reason string) error {
	return fmt.Errorf("%w: %s", domain.ErrUnreachable, reason)
}
