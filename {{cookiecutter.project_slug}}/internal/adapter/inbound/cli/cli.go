// Package cli is the driving adapter: it turns command line input into calls
// on the inbound port and hands the results to a reporter.
//
// It is the only package that knows Cobra exists. The core has no idea it is
// being driven by a CLI, which is why the same use case could later be exposed
// over HTTP or gRPC by adding a sibling adapter and nothing else.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"{{ cookiecutter.module_path }}/internal/adapter/outbound/httpprobe"
	"{{ cookiecutter.module_path }}/internal/adapter/outbound/logging"
	"{{ cookiecutter.module_path }}/internal/adapter/outbound/report"
	"{{ cookiecutter.module_path }}/internal/adapter/outbound/telemetry"
	"{{ cookiecutter.module_path }}/internal/buildinfo"
	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/core/port"
	"{{ cookiecutter.module_path }}/internal/core/service"
	"{{ cookiecutter.module_path }}/internal/observability"
)

// Exit codes. A tool that runs in a pipeline is read by its exit status far
// more often than by its output, so these are part of the public contract.
const (
	// ExitOK means every target met the threshold.
	ExitOK = 0
	// ExitError means the command could not complete: bad input, bad flags.
	ExitError = 1
	// ExitUnhealthy means the command ran fine and the answer was bad news.
	ExitUnhealthy = 2
)

// Options configures a command tree. The adapter fields are the seams the
// functional suite uses: leave them nil for the real thing.
type Options struct {
	// Stdout receives reports; Stderr receives logs and errors.
	Stdout io.Writer
	Stderr io.Writer

	// Build identifies the binary to the user and to the telemetry resource.
	Build buildinfo.Info

	// Prober replaces the HTTP adapter. This is the seam that lets a
	// functional test drive the whole application without a network.
	Prober port.Prober
	// Clock replaces the wall clock inside the HTTP adapter.
	Clock port.Clock
	// Reporter replaces the configured output format entirely.
	Reporter port.Reporter
	// Telemetry, when set, is used as is and never shut down by this package;
	// the caller that supplied it owns its lifecycle.
	Telemetry *observability.Telemetry
}

// app is the resolved wiring for one process. Its fields are populated by
// setup, which runs after Cobra has parsed the flags and before any command
// body executes.
type app struct {
	opts Options

	// Persistent flags.
	outputFormat  string
	timeout       time.Duration
	concurrency   int
	degradedAfter time.Duration
	failOn        string
	logLevel      string
	logFormat     string
	otelProtocol  string
	otelEndpoint  string

	// Resolved collaborators.
	telemetry  *observability.Telemetry
	ownsTel    bool
	logger     port.Logger
	checker    port.HealthChecker
	reporterFn func() (port.Reporter, error)
}

// NewRoot builds the command tree and a cleanup function. The cleanup flushes
// telemetry and must be called even when execution fails, so callers should
// defer it immediately.
func NewRoot(opts Options) (root *cobra.Command, shutdown func(context.Context) error) {
	a := &app{opts: withDefaults(opts)}
	return a.newRootCmd(), a.shutdown
}

func withDefaults(opts Options) Options {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.Build.Name == "" {
		opts.Build = buildinfo.Get()
	}
	return opts
}

func (a *app) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   a.opts.Build.Name,
		Short: "{{ cookiecutter.project_description }}",
		Long: "{{ cookiecutter.project_description }}\n\n" +
			"Built on a hexagonal core: the health rules live in internal/core and\n" +
			"every external system is reached through an adapter behind a port.\n\n" +
			"Telemetry is on by default when an OTLP endpoint is configured. In a\n" +
			"pipeline, set OTEL_EXPORTER_OTLP_ENDPOINT and the run will report its\n" +
			"traces, metrics and logs; a TRACEPARENT in the environment is adopted\n" +
			"as the parent span so the run joins the pipeline's trace.",
		Version:       a.opts.Build.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Wiring happens once, here, for whichever subcommand was chosen.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return a.setup(cmd)
		},
	}

	root.SetOut(a.opts.Stdout)
	root.SetErr(a.opts.Stderr)

	flags := root.PersistentFlags()
	flags.StringVarP(&a.outputFormat, "output", "o", string(report.FormatText),
		"output format: "+joinWords(report.Formats()))
	flags.DurationVar(&a.timeout, "timeout", httpprobe.DefaultTimeout,
		"per-target timeout for a single probe")
	flags.IntVar(&a.concurrency, "concurrency", 8,
		"how many targets to probe at once")
	flags.DurationVar(&a.degradedAfter, "degraded-after", domain.DefaultThresholds().Degraded,
		"latency above which a healthy target is reported as degraded")
	flags.StringVar(&a.failOn, "fail-on", string(failOnDown),
		"exit non-zero when the worst state reaches this: "+joinWords(failOnValues()))
	flags.StringVar(&a.logLevel, "log-level", "info",
		"log level: debug, info, warn, error")
	flags.StringVar(&a.logFormat, "log-format", string(observability.LogFormatText),
		"console log format: "+joinWords(observability.LogFormats()))
	flags.StringVar(&a.otelProtocol, "otel-protocol", "",
		"OpenTelemetry export protocol: "+joinWords(observability.Protocols())+
			" (default from OTEL_EXPORTER_OTLP_PROTOCOL)")
	flags.StringVar(&a.otelEndpoint, "otel-endpoint", "",
		"OpenTelemetry collector endpoint (default from OTEL_EXPORTER_OTLP_ENDPOINT)")

	root.AddCommand(
		a.newCheckCmd(),
		a.newVersionCmd(),
		a.newDocsCmd(),
	)

	return root
}

// setup resolves telemetry and builds the hexagon. Everything the commands
// need is assembled here and nowhere else: this function is the composition
// root, and it is the only place where concrete adapters are named.
func (a *app) setup(cmd *cobra.Command) error {
	ctx := cmd.Context()

	if err := a.setupTelemetry(ctx); err != nil {
		return err
	}
	a.logger = logging.New(a.telemetry.Logger)

	instruments, err := telemetry.NewInstruments(a.telemetry.Meter)
	if err != nil {
		return fmt.Errorf("register metric instruments: %w", err)
	}

	// Outbound: the real HTTP adapter, unless a test supplied its own.
	prober := a.opts.Prober
	if prober == nil {
		probeOpts := []httpprobe.Option{
			httpprobe.WithTimeout(a.timeout),
			httpprobe.WithUserAgent(a.opts.Build.Name + "/" + a.opts.Build.Version),
		}
		if a.opts.Clock != nil {
			probeOpts = append(probeOpts, httpprobe.WithClock(a.opts.Clock))
		}
		prober = httpprobe.New(probeOpts...)
	}
	prober = telemetry.NewProber(prober, a.telemetry.Tracer, instruments, a.logger)

	// Core: the use case, which knows only the ports above.
	checker := service.NewHealth(prober,
		service.WithThresholds(domain.Thresholds{Degraded: a.degradedAfter}),
		service.WithConcurrency(a.concurrency),
	)
	a.checker = telemetry.NewHealthChecker(checker, a.telemetry.Tracer, instruments)

	// Outbound: presentation. Resolved lazily so an unknown --output is
	// reported by the command that needs it rather than by every command.
	a.reporterFn = func() (port.Reporter, error) {
		if a.opts.Reporter != nil {
			return a.opts.Reporter, nil
		}
		return report.New(a.outputFormat, a.opts.Stdout)
	}

	return nil
}

func (a *app) setupTelemetry(ctx context.Context) error {
	if a.opts.Telemetry != nil {
		a.telemetry = a.opts.Telemetry
		return nil
	}

	cfg := observability.ConfigFromEnv(a.opts.Build.Name, a.opts.Build.Version)
	cfg.Stderr = a.opts.Stderr

	if a.otelProtocol != "" {
		protocol, err := observability.ParseProtocol(a.otelProtocol)
		if err != nil {
			return err
		}
		cfg.Protocol = protocol
	}
	if a.otelEndpoint != "" {
		cfg.Endpoint = a.otelEndpoint
		cfg.Insecure = strings.HasPrefix(a.otelEndpoint, "http://")
	}
	level, err := observability.ParseLevel(a.logLevel)
	if err != nil {
		return err
	}
	cfg.LogLevel = level

	format, err := observability.ParseLogFormat(a.logFormat)
	if err != nil {
		return err
	}
	cfg.LogFormat = format

	tel, err := observability.Setup(ctx, cfg)
	if err != nil {
		// Telemetry must never be the reason a pipeline step fails. Fall back
		// to a working no-op SDK and say so on stderr.
		fmt.Fprintf(a.opts.Stderr, "warning: telemetry disabled: %v\n", err)
		tel = observability.Disabled(cfg)
	}

	a.telemetry, a.ownsTel = tel, true
	return nil
}

// shutdown flushes telemetry this package created. Telemetry passed in through
// Options belongs to the caller and is left alone.
func (a *app) shutdown(ctx context.Context) error {
	if a.telemetry == nil || !a.ownsTel {
		return nil
	}
	return a.telemetry.Shutdown(ctx)
}
