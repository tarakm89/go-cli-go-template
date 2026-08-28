package httpprobe_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"{{ cookiecutter.module_path }}/internal/adapter/outbound/fake"
	"{{ cookiecutter.module_path }}/internal/adapter/outbound/httpprobe"
	"{{ cookiecutter.module_path }}/internal/core/domain"
)

func TestProbeReportsStatusAndLatency(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()

	target, err := domain.NewTarget(server.URL)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}

	// A fake clock makes the latency assertion exact instead of flaky.
	clock := fake.NewClock(time.Unix(0, 0), 250*time.Millisecond)
	adapter := httpprobe.New(httpprobe.WithClock(clock))

	probe, err := adapter.Probe(context.Background(), target)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}

	if probe.StatusCode != http.StatusTeapot {
		t.Errorf("StatusCode = %d, want %d", probe.StatusCode, http.StatusTeapot)
	}
	if probe.Latency != 250*time.Millisecond {
		t.Errorf("Latency = %v, want 250ms", probe.Latency)
	}
}

func TestProbeSendsUserAgent(t *testing.T) {
	t.Parallel()

	seen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	target, _ := domain.NewTarget(server.URL)
	adapter := httpprobe.New(httpprobe.WithUserAgent("probe-test/1.2.3"))

	if _, err := adapter.Probe(context.Background(), target); err != nil {
		t.Fatalf("Probe: %v", err)
	}

	if got := <-seen; got != "probe-test/1.2.3" {
		t.Errorf("User-Agent = %q, want %q", got, "probe-test/1.2.3")
	}
}

func TestProbeMapsTransportFailureToDomainError(t *testing.T) {
	t.Parallel()

	// A server that is closed immediately gives us a guaranteed dial failure
	// on a port nobody else is listening on.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	address := server.URL
	server.Close()

	target, _ := domain.NewTarget(address)

	_, err := httpprobe.New(httpprobe.WithTimeout(time.Second)).Probe(context.Background(), target)

	if !errors.Is(err, domain.ErrUnreachable) {
		t.Fatalf("error = %v, want it to wrap domain.ErrUnreachable", err)
	}
}

func TestProbeHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	target, _ := domain.NewTarget(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := httpprobe.New().Probe(ctx, target)

	if !errors.Is(err, domain.ErrUnreachable) {
		t.Fatalf("error = %v, want it to wrap domain.ErrUnreachable", err)
	}
}
