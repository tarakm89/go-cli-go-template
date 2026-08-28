package observability

import (
	"os"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

// DetectCI returns resource attributes describing the pipeline this process is
// running in. Without them a trace from CI is anonymous, and the first
// question anyone asks of a failing run — which commit, which job, which
// branch — cannot be answered from the telemetry.
//
// Nothing here fails: outside CI the slice comes back empty.
func DetectCI() []attribute.KeyValue {
	for _, detect := range []func() []attribute.KeyValue{
		detectGitHubActions,
		detectGitLabCI,
		detectGenericCI,
	} {
		if attrs := detect(); len(attrs) > 0 {
			return attrs
		}
	}
	return nil
}

func detectGitHubActions() []attribute.KeyValue {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return nil
	}
	return collect(
		attribute.String("ci.provider", "github_actions"),
		optional("cicd.pipeline.name", os.Getenv("GITHUB_WORKFLOW")),
		optional("cicd.pipeline.run.id", os.Getenv("GITHUB_RUN_ID")),
		optional("cicd.pipeline.task.name", os.Getenv("GITHUB_JOB")),
		optional("vcs.repository.name", os.Getenv("GITHUB_REPOSITORY")),
		optional("vcs.ref.head.name", os.Getenv("GITHUB_REF_NAME")),
		optional("vcs.ref.head.revision", os.Getenv("GITHUB_SHA")),
	)
}

func detectGitLabCI() []attribute.KeyValue {
	if os.Getenv("GITLAB_CI") != "true" {
		return nil
	}
	return collect(
		attribute.String("ci.provider", "gitlab_ci"),
		optional("cicd.pipeline.name", os.Getenv("CI_PIPELINE_NAME")),
		optional("cicd.pipeline.run.id", os.Getenv("CI_PIPELINE_ID")),
		optional("cicd.pipeline.task.name", os.Getenv("CI_JOB_NAME")),
		optional("vcs.repository.name", os.Getenv("CI_PROJECT_PATH")),
		optional("vcs.ref.head.name", os.Getenv("CI_COMMIT_REF_NAME")),
		optional("vcs.ref.head.revision", os.Getenv("CI_COMMIT_SHA")),
	)
}

// detectGenericCI covers Jenkins, CircleCI, Buildkite and anything else that
// merely sets CI=true.
func detectGenericCI() []attribute.KeyValue {
	if os.Getenv("CI") == "" {
		return nil
	}
	return collect(
		attribute.String("ci.provider", "unknown"),
		optional("cicd.pipeline.name", firstNonEmpty(
			os.Getenv("JOB_NAME"), os.Getenv("CIRCLE_JOB"), os.Getenv("BUILDKITE_PIPELINE_SLUG"))),
		optional("cicd.pipeline.run.id", firstNonEmpty(
			os.Getenv("BUILD_NUMBER"), os.Getenv("CIRCLE_BUILD_NUM"), os.Getenv("BUILDKITE_BUILD_NUMBER"))),
	)
}

// ProcessAttributes describes the invocation itself: which command ran and
// with what arguments.
func ProcessAttributes(command string, args []string) []attribute.KeyValue {
	return collect(
		semconv.ProcessCommand(command),
		attribute.StringSlice("process.command_args", args),
	)
}

// optional yields a zero attribute when the value is empty, so callers can list
// every attribute they might want and let collect filter.
func optional(key, value string) attribute.KeyValue {
	if value == "" {
		return attribute.KeyValue{}
	}
	return attribute.String(key, value)
}

func collect(attrs ...attribute.KeyValue) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		if attr.Valid() {
			out = append(out, attr)
		}
	}
	return out
}
