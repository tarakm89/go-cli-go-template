// Package observability boots the OpenTelemetry SDK — traces, metrics and
// logs — for a short-lived CLI process.
//
// It is infrastructure, not part of the hexagon: the core never imports it.
// Instrumentation reaches the core by decorating its ports, see
// internal/adapter/outbound/telemetry.
//
// Design notes for CI use:
//   - If no OTLP endpoint is configured the SDK falls back to no-op exporters.
//     A pipeline without a collector still runs, and still logs to stderr.
//   - A TRACEPARENT in the environment is adopted as the parent span, so a
//     tool built from this template joins the trace of the pipeline that runs
//     it instead of starting an orphan.
//   - Shutdown always flushes with a bounded timeout, so a wedged collector
//     cannot hang a build.
package observability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Protocol selects how telemetry leaves the process.
type Protocol string

// Supported protocols.
const (
	// ProtocolHTTP is OTLP over HTTP/protobuf, the friendliest option in CI.
	ProtocolHTTP Protocol = "http/protobuf"
	// ProtocolGRPC is OTLP over gRPC, the usual collector port 4317.
	ProtocolGRPC Protocol = "grpc"
	// ProtocolStdout prints telemetry to stderr; useful when debugging.
	ProtocolStdout Protocol = "stdout"
	// ProtocolNone disables export entirely.
	ProtocolNone Protocol = "none"
)

// Protocols lists every supported protocol, for flag help and validation.
func Protocols() []string {
	return []string{
		string(ProtocolHTTP), string(ProtocolGRPC),
		string(ProtocolStdout), string(ProtocolNone),
	}
}

// LogFormat selects the console log encoding.
type LogFormat string

// Supported log formats.
const (
	// LogFormatText is line oriented and meant for a human at a terminal.
	LogFormatText LogFormat = "text"
	// LogFormatJSON is what a CI log collector wants.
	LogFormatJSON LogFormat = "json"
)

// LogFormats lists every supported log format.
func LogFormats() []string { return []string{string(LogFormatText), string(LogFormatJSON)} }

// DefaultShutdownTimeout bounds the final flush.
const DefaultShutdownTimeout = 5 * time.Second

// Config is everything the SDK needs. Build it with ConfigFromEnv and let
// command line flags override individual fields.
type Config struct {
	// ServiceName and ServiceVersion land on the resource of every signal.
	ServiceName    string
	ServiceVersion string

	// Protocol, Endpoint, Insecure and Headers configure the OTLP exporters.
	// An empty Endpoint means "let the SDK read the OTEL_EXPORTER_OTLP_*
	// environment variables"; if those are empty too, export is disabled.
	Protocol Protocol
	Endpoint string
	Insecure bool
	Headers  map[string]string

	// LogLevel and LogFormat configure the console handler on stderr.
	LogLevel  slog.Level
	LogFormat LogFormat

	// ShutdownTimeout bounds the flush of every provider on exit.
	ShutdownTimeout time.Duration

	// Stderr receives console logs and stdout-protocol telemetry.
	Stderr io.Writer
}

// ConfigFromEnv reads the standard OTEL_* environment variables and this
// application's own overrides, returning a Config that is safe to use as is.
func ConfigFromEnv(serviceName, serviceVersion string) Config {
	cfg := Config{
		ServiceName:     firstNonEmpty(os.Getenv("OTEL_SERVICE_NAME"), serviceName),
		ServiceVersion:  serviceVersion,
		Protocol:        ProtocolHTTP,
		Endpoint:        firstNonEmpty(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		Headers:         parseHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")),
		LogLevel:        slog.LevelInfo,
		LogFormat:       LogFormatText,
		ShutdownTimeout: DefaultShutdownTimeout,
		Stderr:          os.Stderr,
	}

	if raw := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); raw != "" {
		if p, err := ParseProtocol(raw); err == nil {
			cfg.Protocol = p
		}
	}
	// OTEL_SDK_DISABLED is part of the specification and is the switch CI
	// operators reach for first.
	if strings.EqualFold(os.Getenv("OTEL_SDK_DISABLED"), "true") {
		cfg.Protocol = ProtocolNone
	}
	if raw := os.Getenv("LOG_LEVEL"); raw != "" {
		if lvl, err := ParseLevel(raw); err == nil {
			cfg.LogLevel = lvl
		}
	}
	if raw := os.Getenv("LOG_FORMAT"); raw != "" {
		if f, err := ParseLogFormat(raw); err == nil {
			cfg.LogFormat = f
		}
	}
	cfg.Insecure = strings.HasPrefix(cfg.Endpoint, "http://")

	return cfg
}

// ParseProtocol validates a protocol name.
func ParseProtocol(raw string) (Protocol, error) {
	switch p := Protocol(strings.ToLower(strings.TrimSpace(raw))); p {
	case ProtocolHTTP, ProtocolGRPC, ProtocolStdout, ProtocolNone:
		return p, nil
	case "http":
		return ProtocolHTTP, nil
	default:
		return "", fmt.Errorf("unknown otel protocol %q (want one of %s)", raw, strings.Join(Protocols(), ", "))
	}
}

// ParseLogFormat validates a console log format.
func ParseLogFormat(raw string) (LogFormat, error) {
	switch f := LogFormat(strings.ToLower(strings.TrimSpace(raw))); f {
	case LogFormatText, LogFormatJSON:
		return f, nil
	default:
		return "", fmt.Errorf("unknown log format %q (want one of %s)", raw, strings.Join(LogFormats(), ", "))
	}
}

// ParseLevel validates a log level.
func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q (want one of debug, info, warn, error)", raw)
	}
}

// exportEnabled reports whether any exporter should be constructed. Falling
// back to no-op when nothing is configured is what keeps this usable in a
// pipeline that has no collector.
func (c Config) exportEnabled() bool {
	switch c.Protocol {
	case ProtocolNone:
		return false
	case ProtocolStdout:
		return true
	case ProtocolHTTP, ProtocolGRPC:
		return c.Endpoint != "" ||
			os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
			os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseHeaders decodes the W3C-ish `key=value,key=value` form used by
// OTEL_EXPORTER_OTLP_HEADERS.
func parseHeaders(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	headers := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key != "" {
			headers[key] = value
		}
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}
