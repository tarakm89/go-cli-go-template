package functional_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"{{ cookiecutter.module_path }}/internal/adapter/inbound/cli"
	"{{ cookiecutter.module_path }}/internal/adapter/outbound/fake"
)

const address = "https://api.example.com"

// checkCase is one row of the grading table. Naming the fields keeps the
// entries readable as sentences, which is the whole point of writing them as
// specs rather than as a Go test table.
type checkCase struct {
	// Response is what the faked external system returns.
	Response fake.Response
	// Flags are added to the command line before the target.
	Flags []string
	// WantExit is the process exit code the pipeline would observe.
	WantExit int
	// WantSummary is the verdict printed on the summary line.
	WantSummary string
}

var _ = Describe("check", func() {
	var app *harness

	BeforeEach(func() {
		app = newHarness()
		DeferCleanup(app.Close)
	})

	// The grading rules and their consequences for a pipeline, as a table.
	DescribeTable("grading a target",
		func(tc checkCase) {
			app.Prober.With(address, tc.Response)

			args := append(append([]string{"check"}, tc.Flags...), address)

			Expect(app.Run(args...)).To(Equal(tc.WantExit))
			Expect(app.Stdout()).To(ContainSubstring("summary: " + tc.WantSummary))
		},

		Entry("a fast 200 is up, and the run succeeds", checkCase{
			Response:    fake.Response{StatusCode: 200, Latency: 10 * time.Millisecond},
			WantExit:    cli.ExitOK,
			WantSummary: "up",
		}),
		Entry("a redirect is still up", checkCase{
			Response:    fake.Response{StatusCode: 301, Latency: 10 * time.Millisecond},
			WantExit:    cli.ExitOK,
			WantSummary: "up",
		}),
		Entry("a 404 is degraded, but does not fail the run by default", checkCase{
			Response:    fake.Response{StatusCode: 404, Latency: 10 * time.Millisecond},
			WantExit:    cli.ExitOK,
			WantSummary: "degraded",
		}),
		Entry("a 404 does fail the run under --fail-on degraded", checkCase{
			Response:    fake.Response{StatusCode: 404, Latency: 10 * time.Millisecond},
			Flags:       []string{"--fail-on", "degraded"},
			WantExit:    cli.ExitUnhealthy,
			WantSummary: "degraded",
		}),
		Entry("a 500 is down and fails the run", checkCase{
			Response:    fake.Response{StatusCode: 500, Latency: 10 * time.Millisecond},
			WantExit:    cli.ExitUnhealthy,
			WantSummary: "down",
		}),
		Entry("an unreachable target is down", checkCase{
			Response:    fake.Response{Err: unreachable("connection refused")},
			WantExit:    cli.ExitUnhealthy,
			WantSummary: "down",
		}),
		Entry("a slow response is degraded once the budget is tightened", checkCase{
			Response:    fake.Response{StatusCode: 200, Latency: 800 * time.Millisecond},
			Flags:       []string{"--degraded-after", "100ms"},
			WantExit:    cli.ExitOK,
			WantSummary: "degraded",
		}),
		Entry("the same slow response is up under a generous budget", checkCase{
			Response:    fake.Response{StatusCode: 200, Latency: 800 * time.Millisecond},
			Flags:       []string{"--degraded-after", "2s"},
			WantExit:    cli.ExitOK,
			WantSummary: "up",
		}),
		Entry("--fail-on never keeps the run green whatever happens", checkCase{
			Response:    fake.Response{StatusCode: 500, Latency: 10 * time.Millisecond},
			Flags:       []string{"--fail-on", "never"},
			WantExit:    cli.ExitOK,
			WantSummary: "down",
		}),
	)

	Context("with several targets", func() {
		BeforeEach(func() {
			app.Prober.With("https://a.example.com", fake.Response{StatusCode: 200, Latency: 10 * time.Millisecond})
			app.Prober.With("https://b.example.com", fake.Response{StatusCode: 200, Latency: 20 * time.Millisecond})
			app.Prober.WithFailure("https://c.example.com", "connection refused")
		})

		It("probes each of them exactly once", func() {
			app.Run("check", "--fail-on", "never",
				"https://a.example.com", "https://b.example.com", "https://c.example.com")

			Expect(app.Prober.Calls()).To(HaveLen(3))
		})

		It("reports every target and summarises with the worst state", func() {
			code := app.Run("check", "https://a.example.com", "https://b.example.com", "https://c.example.com")

			Expect(code).To(Equal(cli.ExitUnhealthy))
			Expect(app.Stdout()).To(ContainSubstring("a.example.com"))
			Expect(app.Stdout()).To(ContainSubstring("b.example.com"))
			Expect(app.Stdout()).To(ContainSubstring("connection refused"))
			Expect(app.Stdout()).To(ContainSubstring("summary: down"))
		})
	})

	Describe("json output", func() {
		It("emits a document a pipeline can parse", func() {
			app.Prober.With(address, fake.Response{StatusCode: 503, Latency: 5 * time.Millisecond})

			Expect(app.Run("check", "--output", "json", address)).To(Equal(cli.ExitUnhealthy))

			var doc struct {
				Summary string `json:"summary"`
				Checks  []struct {
					Target string `json:"target"`
					State  string `json:"state"`
				} `json:"checks"`
			}
			Expect(json.Unmarshal([]byte(app.Stdout()), &doc)).To(Succeed())
			Expect(doc.Summary).To(Equal("down"))
			Expect(doc.Checks).To(HaveLen(1))
			Expect(doc.Checks[0].Target).To(Equal("api.example.com"))
			Expect(doc.Checks[0].State).To(Equal("down"))
		})
	})

	DescribeTable("rejecting bad input",
		func(args []string, wantMessage string) {
			Expect(app.Run(args...)).To(Equal(cli.ExitError))
			Expect(app.Stderr()).To(ContainSubstring(wantMessage))
			Expect(app.Prober.CallCount()).To(BeZero(),
				"nothing should be probed until the whole command line is valid")
		},

		Entry("a target that is not a url",
			[]string{"check", "https://good.example.com", "not-a-url"}, "invalid target"),
		Entry("a target with an unsupported scheme",
			[]string{"check", "ftp://example.com"}, "scheme must be http or https"),
		Entry("an unknown output format",
			[]string{"check", "--output", "yaml", address}, "unknown output format"),
		Entry("an unknown --fail-on value",
			[]string{"check", "--fail-on", "sometimes", address}, "unknown --fail-on value"),
		Entry("no targets at all",
			[]string{"check"}, "requires at least 1 arg"),
	)
})
