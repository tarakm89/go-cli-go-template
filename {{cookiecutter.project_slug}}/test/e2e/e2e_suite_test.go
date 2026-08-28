// Package e2e_test exercises the compiled binary.
//
// Where the functional suite swaps in fake adapters, this one builds the real
// executable and points it at a real HTTP server, so the code paths that only
// exist in a shipped binary — flag parsing, exit codes, the HTTP adapter,
// telemetry shutdown — are covered end to end.
package e2e_test

import (
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

// binaryPath is the compiled CLI under test, shared by every parallel node.
var binaryPath string

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Runs once, on node 1: build the binary the whole suite will exercise.
	path, err := gexec.Build("{{ cookiecutter.module_path }}/cmd/{{ cookiecutter.binary_name }}")
	Expect(err).NotTo(HaveOccurred())
	return []byte(path)
}, func(path []byte) {
	binaryPath = string(path)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

// run executes the CLI and waits for it to finish.
func run(args ...string) *gexec.Session {
	command := exec.Command(binaryPath, args...)
	// Telemetry off by default: these specs assert on the tool's own output,
	// and a missing collector should never slow the suite down.
	command.Env = append(command.Environ(), "OTEL_SDK_DISABLED=true")

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, 30*time.Second).Should(gexec.Exit())

	return session
}
