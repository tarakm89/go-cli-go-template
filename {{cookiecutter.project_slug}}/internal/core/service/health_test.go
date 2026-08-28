package service_test

import (
	"context"
	"testing"
	"time"

	"{{ cookiecutter.module_path }}/internal/adapter/outbound/fake"
	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/core/service"
)

// The use case is exercised entirely through the fake adapter: no network, no
// clock, no output. That is the point of the port.

func mustTarget(t *testing.T, raw string) domain.Target {
	t.Helper()
	target, err := domain.NewTarget(raw)
	if err != nil {
		t.Fatalf("NewTarget(%q): %v", raw, err)
	}
	return target
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	target := mustTarget(t, "https://api.example.com")

	tests := []struct {
		name     string
		response fake.Response
		want     domain.State
	}{
		{name: "healthy", response: fake.Response{StatusCode: 200, Latency: 5 * time.Millisecond}, want: domain.StateUp},
		{name: "slow", response: fake.Response{StatusCode: 200, Latency: 2 * time.Second}, want: domain.StateDegraded},
		{name: "server error", response: fake.Response{StatusCode: 502, Latency: time.Millisecond}, want: domain.StateDown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			prober := fake.NewProber().With(target.Address, test.response)
			checker := service.NewHealth(prober)

			got := checker.Check(context.Background(), target)

			if got.State != test.want {
				t.Errorf("Check().State = %q, want %q", got.State, test.want)
			}
			if prober.CallCount() != 1 {
				t.Errorf("prober called %d times, want 1", prober.CallCount())
			}
		})
	}
}

func TestHealthCheckUnreachableIsAVerdictNotAnError(t *testing.T) {
	t.Parallel()

	target := mustTarget(t, "https://down.example.com")
	prober := fake.NewProber().WithFailure(target.Address, "connection refused")

	got := service.NewHealth(prober).Check(context.Background(), target)

	if got.State != domain.StateDown {
		t.Errorf("State = %q, want %q", got.State, domain.StateDown)
	}
	if got.Detail == "" {
		t.Error("an unreachable target must explain itself")
	}
}

func TestHealthCheckAllPreservesOrder(t *testing.T) {
	t.Parallel()

	addresses := []string{
		"https://a.example.com",
		"https://b.example.com",
		"https://c.example.com",
	}

	prober := fake.NewProber()
	targets := make([]domain.Target, 0, len(addresses))
	for i, address := range addresses {
		target := mustTarget(t, address)
		targets = append(targets, target)
		// Reverse the delays so a naive implementation that appends results as
		// they arrive would come back in the wrong order.
		prober.With(address, fake.Response{
			StatusCode: 200,
			Latency:    time.Millisecond,
			Delay:      time.Duration(len(addresses)-i) * 10 * time.Millisecond,
		})
	}

	results := service.NewHealth(prober, service.WithConcurrency(3)).
		CheckAll(context.Background(), targets)

	if len(results) != len(targets) {
		t.Fatalf("got %d results, want %d", len(results), len(targets))
	}
	for i, result := range results {
		if result.Target.Address != addresses[i] {
			t.Errorf("result %d is for %q, want %q", i, result.Target.Address, addresses[i])
		}
	}
}

func TestHealthCheckAllRespectsCancellation(t *testing.T) {
	t.Parallel()

	target := mustTarget(t, "https://slow.example.com")
	prober := fake.NewProber().With(target.Address, fake.Response{
		StatusCode: 200,
		Delay:      30 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	results := service.NewHealth(prober).CheckAll(ctx, []domain.Target{target})
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("CheckAll ignored cancellation; it took %v", elapsed)
	}
	if len(results) != 1 || results[0].State != domain.StateDown {
		t.Fatalf("cancelled probe should be reported down, got %+v", results)
	}
}

func TestHealthCheckAllEmpty(t *testing.T) {
	t.Parallel()

	results := service.NewHealth(fake.NewProber()).CheckAll(context.Background(), nil)

	if len(results) != 0 {
		t.Errorf("got %d results for no targets, want 0", len(results))
	}
}

func TestHealthThresholdsAreConfigurable(t *testing.T) {
	t.Parallel()

	target := mustTarget(t, "https://api.example.com")
	prober := fake.NewProber().With(target.Address, fake.Response{
		StatusCode: 200,
		Latency:    100 * time.Millisecond,
	})

	strict := service.NewHealth(prober, service.WithThresholds(domain.Thresholds{Degraded: 50 * time.Millisecond}))
	if got := strict.Check(context.Background(), target); got.State != domain.StateDegraded {
		t.Errorf("with a 50ms budget, 100ms should be degraded, got %q", got.State)
	}

	lenient := service.NewHealth(prober, service.WithThresholds(domain.Thresholds{Degraded: time.Second}))
	if got := lenient.Check(context.Background(), target); got.State != domain.StateUp {
		t.Errorf("with a 1s budget, 100ms should be up, got %q", got.State)
	}
}
