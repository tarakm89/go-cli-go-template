package e2e_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("check against a real server", func() {
	var (
		server *httptest.Server
		status int
	)

	BeforeEach(func() {
		status = http.StatusOK
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		DeferCleanup(server.Close)
	})

	// The same grading table as the functional suite, but through the real
	// binary and a real HTTP round trip. If these two ever disagree, the fake
	// adapter has drifted from the real one.
	DescribeTable("mapping an HTTP status to an exit code",
		func(responseStatus, wantExit int, wantSummary string) {
			status = responseStatus

			session := run("check", server.URL)

			Expect(session).To(gexec.Exit(wantExit))
			Expect(session.Out).To(gbytes.Say("summary: " + wantSummary))
		},

		Entry("200 is up and the run succeeds", http.StatusOK, 0, "up"),
		Entry("301 is up", http.StatusMovedPermanently, 0, "up"),
		Entry("404 is degraded but does not fail the run", http.StatusNotFound, 0, "degraded"),
		Entry("500 is down and fails the run", http.StatusInternalServerError, 2, "down"),
		Entry("503 is down", http.StatusServiceUnavailable, 2, "down"),
	)

	It("emits parseable json", func() {
		session := run("check", "--output", "json", server.URL)

		Expect(session).To(gexec.Exit(0))

		var doc struct {
			Summary string `json:"summary"`
			Checks  []struct {
				State   string `json:"state"`
				Address string `json:"address"`
			} `json:"checks"`
		}
		Expect(json.Unmarshal(session.Out.Contents(), &doc)).To(Succeed())
		Expect(doc.Summary).To(Equal("up"))
		Expect(doc.Checks).To(HaveLen(1))
		Expect(doc.Checks[0].Address).To(Equal(server.URL))
	})

	It("reports an unreachable server without crashing", func() {
		address := server.URL
		server.Close()

		session := run("check", address)

		Expect(session).To(gexec.Exit(2))
		Expect(session.Out).To(gbytes.Say("summary: down"))
	})

	It("checks several targets in one run", func() {
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		DeferCleanup(other.Close)

		session := run("check", "--output", "json", server.URL, other.URL)

		Expect(session).To(gexec.Exit(2))

		var doc struct {
			Checks []struct {
				State string `json:"state"`
			} `json:"checks"`
		}
		Expect(json.Unmarshal(session.Out.Contents(), &doc)).To(Succeed())
		Expect(doc.Checks).To(HaveLen(2))
		Expect(doc.Checks[0].State).To(Equal("up"))
		Expect(doc.Checks[1].State).To(Equal("down"))
	})
})

var _ = Describe("the binary itself", func() {
	DescribeTable("rejecting bad input",
		func(args []string, wantMessage string) {
			session := run(args...)

			Expect(session).To(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say(wantMessage))
		},

		Entry("a malformed target", []string{"check", "not-a-url"}, "invalid target"),
		Entry("an unsupported scheme", []string{"check", "ftp://example.com"}, "scheme must be http or https"),
		Entry("an unknown output format", []string{"check", "--output", "yaml", "https://example.com"}, "unknown output format"),
		Entry("an unknown subcommand", []string{"nonsense"}, "unknown command"),
	)

	It("prints its version", func() {
		session := run("version")

		Expect(session).To(gexec.Exit(0))
		Expect(session.Out).To(gbytes.Say(`\S`))
	})

	It("prints build information as json", func() {
		session := run("version", "--json")

		Expect(session).To(gexec.Exit(0))

		var info struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			GoVersion string `json:"go_version"`
		}
		Expect(json.Unmarshal(session.Out.Contents(), &info)).To(Succeed())
		Expect(info.Name).NotTo(BeEmpty())
		Expect(info.GoVersion).To(HavePrefix("go"))
	})

	It("prints help without an error", func() {
		session := run("--help")

		Expect(session).To(gexec.Exit(0))
		Expect(session.Out).To(gbytes.Say("check"))
	})
})
