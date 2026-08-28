package observability_test

import (
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"{{ cookiecutter.module_path }}/internal/observability"
)

func TestParseProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    observability.Protocol
		wantErr bool
	}{
		{name: "http/protobuf", input: "http/protobuf", want: observability.ProtocolHTTP},
		{name: "http is an alias", input: "http", want: observability.ProtocolHTTP},
		{name: "grpc", input: "grpc", want: observability.ProtocolGRPC},
		{name: "stdout", input: "stdout", want: observability.ProtocolStdout},
		{name: "none", input: "none", want: observability.ProtocolNone},
		{name: "case and space insensitive", input: "  GRPC ", want: observability.ProtocolGRPC},
		{name: "anything else is rejected", input: "carrier-pigeon", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := observability.ParseProtocol(test.input)

			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseProtocol(%q) should have failed", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseProtocol(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("ParseProtocol(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    slog.Level
		wantErr bool
	}{
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "info", input: "INFO", want: slog.LevelInfo},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "warning is an alias", input: "warning", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "anything else is rejected", input: "chatty", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := observability.ParseLevel(test.input)

			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseLevel(%q) should have failed", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLevel(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

// A pipeline with no collector must still be able to run the tool, so the
// no-export path is a supported configuration rather than a failure mode.
func TestConfigFromEnvWithoutACollector(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_SDK_DISABLED", "")

	cfg := observability.ConfigFromEnv("probe", "1.0.0")

	tel, err := observability.Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup should succeed with no collector configured: %v", err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })

	if tel.Exporting {
		t.Error("nothing is configured, so nothing should be exported")
	}
	if tel.Tracer == nil || tel.Meter == nil || tel.Logger == nil {
		t.Error("a disabled SDK must still hand back usable no-op instruments")
	}
}

func TestConfigFromEnvHonoursSDKDisabled(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_SDK_DISABLED", "true")

	if got := observability.ConfigFromEnv("probe", "1.0.0").Protocol; got != observability.ProtocolNone {
		t.Errorf("Protocol = %q, want %q when OTEL_SDK_DISABLED is set", got, observability.ProtocolNone)
	}
}

func TestConfigFromEnvReadsStandardVariables(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "renamed")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization=Bearer token, x-tenant = acme")
	t.Setenv("OTEL_SDK_DISABLED", "")

	cfg := observability.ConfigFromEnv("probe", "1.0.0")

	if cfg.ServiceName != "renamed" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "renamed")
	}
	if cfg.Protocol != observability.ProtocolGRPC {
		t.Errorf("Protocol = %q, want %q", cfg.Protocol, observability.ProtocolGRPC)
	}
	if !cfg.Insecure {
		t.Error("an http:// endpoint should be treated as insecure")
	}
	if cfg.Headers["authorization"] != "Bearer token" {
		t.Errorf("headers = %v, want an authorization entry", cfg.Headers)
	}
	if cfg.Headers["x-tenant"] != "acme" {
		t.Errorf("headers = %v, want whitespace around the pair to be trimmed", cfg.Headers)
	}
}

// The CI attributes are what make a trace from a pipeline identifiable, so it
// is worth asserting that the detector actually fires.
func TestDetectCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_WORKFLOW", "CI")
	t.Setenv("GITHUB_RUN_ID", "42")
	t.Setenv("GITHUB_REPOSITORY", "acme/probe")
	t.Setenv("GITHUB_REF_NAME", "main")
	t.Setenv("GITHUB_SHA", "")

	attrs := make(map[attribute.Key]string)
	for _, attr := range observability.DetectCI() {
		attrs[attr.Key] = attr.Value.AsString()
	}

	if attrs["ci.provider"] != "github_actions" {
		t.Errorf("ci.provider = %q, want %q", attrs["ci.provider"], "github_actions")
	}
	if attrs["cicd.pipeline.run.id"] != "42" {
		t.Errorf("cicd.pipeline.run.id = %q, want %q", attrs["cicd.pipeline.run.id"], "42")
	}
	if _, present := attrs["vcs.ref.head.revision"]; present {
		t.Error("an empty environment variable should not become an attribute")
	}
}

func TestDetectCIOutsideCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI", "")

	if attrs := observability.DetectCI(); len(attrs) != 0 {
		t.Errorf("outside CI there should be no attributes, got %v", attrs)
	}
}

func TestContextFromEnvironmentAdoptsTraceparent(t *testing.T) {
	t.Setenv("TRACEPARENT", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	tel := observability.Disabled(observability.Config{})

	ctx := tel.ContextFromEnvironment(context.Background())

	injected := tel.InjectIntoEnvironment(ctx)
	if len(injected) == 0 {
		t.Fatal("a TRACEPARENT in the environment should have been adopted")
	}
	want := "TRACEPARENT=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if injected[0] != want {
		t.Errorf("injected %q, want %q", injected[0], want)
	}
}
