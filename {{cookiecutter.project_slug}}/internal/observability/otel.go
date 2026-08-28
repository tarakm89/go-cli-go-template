package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Telemetry is the handle a wired application holds onto. Everything on it is
// safe to use even when export is disabled: the providers are then no-ops.
type Telemetry struct {
	// Tracer, Meter and Logger are scoped to this application.
	Tracer trace.Tracer
	Meter  metric.Meter
	Logger *slog.Logger

	// Propagator carries trace context in and out of the process, which is
	// how a CLI joins the trace of the pipeline step that invoked it.
	Propagator propagation.TextMapPropagator

	// Exporting reports whether telemetry actually leaves the process.
	Exporting bool

	shutdownTimeout time.Duration
	shutdownFuncs   []func(context.Context) error
}

// Setup builds the SDK from cfg and installs it as the process-wide default.
//
// It never returns a partially installed SDK: if any provider fails to build,
// everything already created is shut down and the error is returned. Callers
// that would rather degrade than fail should log the error and call Disabled.
func Setup(ctx context.Context, cfg Config) (*Telemetry, error) {
	cfg = withDefaults(cfg)

	tel := &Telemetry{
		Propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
		shutdownTimeout: cfg.ShutdownTimeout,
		Exporting:       cfg.exportEnabled(),
	}
	otel.SetTextMapPropagator(tel.Propagator)

	// Never let an SDK problem write to the user's output; route it to the
	// console logger at debug level instead.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		if tel.Logger != nil {
			tel.Logger.Debug("opentelemetry error", slog.String("error", err.Error()))
		}
	}))

	if !tel.Exporting {
		tel.installNoop(cfg)
		return tel, nil
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	tracerProvider, err := newTracerProvider(ctx, cfg, res)
	if err != nil {
		return nil, tel.abort(ctx, fmt.Errorf("build tracer provider: %w", err))
	}
	tel.shutdownFuncs = append(tel.shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	meterProvider, err := newMeterProvider(ctx, cfg, res)
	if err != nil {
		return nil, tel.abort(ctx, fmt.Errorf("build meter provider: %w", err))
	}
	tel.shutdownFuncs = append(tel.shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	loggerProvider, err := newLoggerProvider(ctx, cfg, res)
	if err != nil {
		return nil, tel.abort(ctx, fmt.Errorf("build logger provider: %w", err))
	}
	tel.shutdownFuncs = append(tel.shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

	tel.Tracer = tracerProvider.Tracer(cfg.ServiceName)
	tel.Meter = meterProvider.Meter(cfg.ServiceName)
	tel.Logger = newLogger(cfg, loggerProvider)

	return tel, nil
}

// Disabled returns a fully functional handle that exports nothing. Use it when
// the SDK could not be built, or in tests that do not care about telemetry.
func Disabled(cfg Config) *Telemetry {
	cfg = withDefaults(cfg)
	tel := &Telemetry{
		Propagator:      propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		shutdownTimeout: cfg.ShutdownTimeout,
	}
	tel.installNoop(cfg)
	return tel
}

func (t *Telemetry) installNoop(cfg Config) {
	t.Tracer = tracenoop.NewTracerProvider().Tracer(cfg.ServiceName)
	t.Meter = metricnoop.NewMeterProvider().Meter(cfg.ServiceName)
	t.Logger = newLogger(cfg, lognoop.NewLoggerProvider())
	t.Exporting = false
}

// abort tears down whatever was already installed and returns cause.
func (t *Telemetry) abort(ctx context.Context, cause error) error {
	if err := t.Shutdown(ctx); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// Shutdown flushes every provider. It is safe to call more than once, and it
// always returns within the configured timeout so a wedged collector cannot
// hang a pipeline.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if len(t.shutdownFuncs) == 0 {
		return nil
	}

	// Detach from the caller's context: shutdown usually runs while the
	// command context is already cancelled, and the final flush still has to
	// happen.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), t.shutdownTimeout)
	defer cancel()

	var errs []error
	for i := len(t.shutdownFuncs) - 1; i >= 0; i-- {
		if err := t.shutdownFuncs[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	t.shutdownFuncs = nil

	return errors.Join(errs...)
}

// ContextFromEnvironment adopts a TRACEPARENT/TRACESTATE pair published by the
// surrounding CI system, so this process becomes a child span of the pipeline
// step rather than the root of its own trace.
func (t *Telemetry) ContextFromEnvironment(ctx context.Context) context.Context {
	carrier := propagation.MapCarrier{}
	for _, key := range []string{"traceparent", "tracestate"} {
		if value := firstNonEmpty(os.Getenv(strings.ToUpper(key)), os.Getenv(key)); value != "" {
			carrier[key] = value
		}
	}
	if len(carrier) == 0 {
		return ctx
	}
	return t.Propagator.Extract(ctx, carrier)
}

// InjectIntoEnvironment renders the current span context as environment
// variables, ready to hand to a child process so the trace keeps going.
func (t *Telemetry) InjectIntoEnvironment(ctx context.Context) []string {
	carrier := propagation.MapCarrier{}
	t.Propagator.Inject(ctx, carrier)

	env := make([]string, 0, len(carrier))
	for key, value := range carrier {
		env = append(env, strings.ToUpper(key)+"="+value)
	}
	sort.Strings(env)
	return env
}

func withDefaults(cfg Config) Config {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "{{ cookiecutter.binary_name }}"
	}
	if cfg.Protocol == "" {
		cfg.Protocol = ProtocolNone
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = LogFormatText
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	return cfg
}

// buildResource describes *what* is emitting telemetry. Detected CI attributes
// are what make the signals useful once they land in a backend.
func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	}
	attrs = append(attrs, DetectCI()...)

	return resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithFromEnv(), // honours OTEL_RESOURCE_ATTRIBUTES
		resource.WithAttributes(attrs...),
	)
}

func newTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		// A CLI run is short and rare; sampling it away loses the only trace
		// anyone will ever look at. Honour an inherited decision, though.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithBatcher(exporter),
	), nil
}

func newMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := newMetricExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		// The process usually exits long before any interval elapses; the
		// flush on shutdown is what actually delivers the metrics.
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(15*time.Second))),
	), nil
}

func newLoggerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exporter, err := newLogExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	), nil
}

// newLogger fans slog records out to the console and to the OTLP log pipeline,
// so an operator tailing CI output and a backend query see the same events.
func newLogger(cfg Config, provider otellog.LoggerProvider) *slog.Logger {
	handlers := []slog.Handler{consoleHandler(cfg)}
	if provider != nil {
		handlers = append(handlers, otelslog.NewHandler(cfg.ServiceName,
			otelslog.WithLoggerProvider(provider)))
	}
	return slog.New(newFanoutHandler(handlers...))
}

// WithProviders builds a handle around providers the caller already owns.
//
// This is the seam the functional suite uses: it hands in an in-memory span
// exporter and a manual metric reader, then asserts on the telemetry the run
// produced. Shutdown is a no-op here — whoever supplied the providers is
// responsible for flushing them.
func WithProviders(name string, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider, logger *slog.Logger) *Telemetry {
	if name == "" {
		name = "{{ cookiecutter.binary_name }}"
	}
	if tracerProvider == nil {
		tracerProvider = tracenoop.NewTracerProvider()
	}
	if meterProvider == nil {
		meterProvider = metricnoop.NewMeterProvider()
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Telemetry{
		Tracer:          tracerProvider.Tracer(name),
		Meter:           meterProvider.Meter(name),
		Logger:          logger,
		Propagator:      propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		shutdownTimeout: DefaultShutdownTimeout,
	}
}
