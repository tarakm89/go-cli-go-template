// Package functional_test drives the whole application in-process.
//
// It wires the real command tree, the real use case and the real reporters,
// and replaces only the adapters that would otherwise reach an external
// system. Nothing here touches the network, so the suite is fast and
// deterministic enough to run on every commit — and because it goes through
// bootstrap.Run, it asserts on the exact exit code the shipped binary returns.
package functional_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Suite")
}
