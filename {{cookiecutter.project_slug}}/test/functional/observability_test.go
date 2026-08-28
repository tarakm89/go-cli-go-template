package functional_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"{{ cookiecutter.module_path }}/internal/adapter/outbound/fake"
)

// Telemetry is a feature of this tool, not a side effect, so it gets specs of
// its own. If a refactor stops emitting spans the pipeline dashboards go
// quiet, and that should fail the build here rather than in production.
var _ = Describe("observability", func() {
	var app *harness

	BeforeEach(func() {
		app = newHarness()
		DeferCleanup(app.Close)

		app.Prober.With("https://a.example.com", fake.Response{StatusCode: 200, Latency: 10 * time.Millisecond})
		app.Prober.WithFailure("https://down.example.com", "connection refused")
	})

	It("traces the run and every external call", func() {
		Expect(app.Run("check", "--fail-on", "never", "https://a.example.com")).To(BeZero())

		Expect(app.SpanNames()).To(ContainElements(
			"check all",
			"probe a.example.com",
		), "the run and every external call should be traced")
	})

	It("hangs each external call off the run that made it", func() {
		Expect(app.Run("check", "--fail-on", "never", "https://a.example.com")).To(BeZero())

		// A flat list of spans is not much use in a backend; what makes a
		// slow pipeline step diagnosable is the parent/child shape.
		Expect(app.ParentOf("probe a.example.com")).To(Equal("check all"))
	})

	It("records the verdict on the run span", func() {
		Expect(app.Run("check", "--fail-on", "never", "https://down.example.com")).To(BeZero())

		span, found := app.SpanNamed("check all")
		Expect(found).To(BeTrue())

		attrs := make(map[string]string)
		for _, attr := range span.Attributes {
			attrs[string(attr.Key)] = attr.Value.String()
		}
		Expect(attrs).To(HaveKeyWithValue("check.summary", "down"))
	})

	It("records probe and check metrics", func() {
		Expect(app.Run("check", "--fail-on", "never", "https://a.example.com")).To(BeZero())

		Expect(app.MetricNames()).To(ContainElements("probe.duration", "probe.total", "check.total"))
	})

	It("traces a failed call too", func() {
		Expect(app.Run("check", "--fail-on", "never", "https://down.example.com")).To(BeZero())

		Expect(app.SpanNames()).To(ContainElement("probe down.example.com"))
	})

	It("logs the start and the end of a run", func() {
		Expect(app.Run("check", "--fail-on", "never", "https://a.example.com")).To(BeZero())

		Expect(app.Stderr()).To(ContainSubstring("starting health check"))
		Expect(app.Stderr()).To(ContainSubstring("health check finished"))
	})
})
