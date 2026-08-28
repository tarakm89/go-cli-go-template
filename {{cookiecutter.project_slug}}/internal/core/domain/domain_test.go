package domain_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"{{ cookiecutter.module_path }}/internal/core/domain"
)

func TestNewTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantName string
		wantErr  error
	}{
		{name: "https url", input: "https://api.example.com/healthz", wantName: "api.example.com"},
		{name: "http url", input: "http://localhost:8080/health", wantName: "localhost:8080"},
		{name: "surrounding whitespace", input: "  https://example.com  ", wantName: "example.com"},
		{name: "empty", input: "", wantErr: domain.ErrInvalidTarget},
		{name: "missing scheme", input: "example.com", wantErr: domain.ErrInvalidTarget},
		{name: "unsupported scheme", input: "ftp://example.com", wantErr: domain.ErrInvalidTarget},
		{name: "missing host", input: "https://", wantErr: domain.ErrInvalidTarget},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target, err := domain.NewTarget(test.input)

			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("NewTarget(%q) error = %v, want %v", test.input, err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTarget(%q) unexpected error: %v", test.input, err)
			}
			if target.Name != test.wantName {
				t.Errorf("NewTarget(%q).Name = %q, want %q", test.input, target.Name, test.wantName)
			}
		})
	}
}

func TestThresholdsEvaluate(t *testing.T) {
	t.Parallel()

	thresholds := domain.Thresholds{Degraded: 500 * time.Millisecond}
	target := domain.Target{Name: "example.com", Address: "https://example.com"}

	tests := []struct {
		name  string
		probe domain.Probe
		want  domain.State
	}{
		{name: "fast 200 is up", probe: domain.Probe{StatusCode: 200, Latency: 10 * time.Millisecond}, want: domain.StateUp},
		{name: "300 is up", probe: domain.Probe{StatusCode: 301, Latency: time.Millisecond}, want: domain.StateUp},
		{name: "slow 200 is degraded", probe: domain.Probe{StatusCode: 200, Latency: 900 * time.Millisecond}, want: domain.StateDegraded},
		{name: "404 is degraded", probe: domain.Probe{StatusCode: 404, Latency: time.Millisecond}, want: domain.StateDegraded},
		{name: "500 is down", probe: domain.Probe{StatusCode: 500, Latency: time.Millisecond}, want: domain.StateDown},
		{name: "503 is down even when fast", probe: domain.Probe{StatusCode: 503, Latency: time.Nanosecond}, want: domain.StateDown},
		{name: "server error beats slowness", probe: domain.Probe{StatusCode: 500, Latency: time.Hour}, want: domain.StateDown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := thresholds.Evaluate(target, test.probe)

			if got.State != test.want {
				t.Errorf("Evaluate(%+v).State = %q, want %q", test.probe, got.State, test.want)
			}
			if got.Detail == "" {
				t.Error("Evaluate produced an empty Detail; the user needs a reason")
			}
			if got.Latency != test.probe.Latency {
				t.Errorf("Evaluate lost the latency: got %v, want %v", got.Latency, test.probe.Latency)
			}
		})
	}
}

func TestUnreachable(t *testing.T) {
	t.Parallel()

	target := domain.Target{Name: "example.com", Address: "https://example.com"}

	got := domain.Unreachable(target, errors.New("dial tcp: connection refused"))
	if got.State != domain.StateDown {
		t.Errorf("State = %q, want %q", got.State, domain.StateDown)
	}
	if got.Detail != "dial tcp: connection refused" {
		t.Errorf("Detail = %q, want the cause", got.Detail)
	}

	if got := domain.Unreachable(target, nil); got.Detail == "" {
		t.Error("a nil cause must still produce a Detail")
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		states []domain.State
		want   domain.State
	}{
		{name: "empty is up", states: nil, want: domain.StateUp},
		{name: "all up", states: []domain.State{domain.StateUp, domain.StateUp}, want: domain.StateUp},
		{name: "one degraded", states: []domain.State{domain.StateUp, domain.StateDegraded}, want: domain.StateDegraded},
		{name: "down wins over degraded", states: []domain.State{domain.StateDegraded, domain.StateDown}, want: domain.StateDown},
		{name: "down wins regardless of order", states: []domain.State{domain.StateDown, domain.StateUp}, want: domain.StateDown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			checks := make([]domain.Health, 0, len(test.states))
			for _, state := range test.states {
				checks = append(checks, domain.Health{State: state})
			}

			if got := domain.Summary(checks); got != test.want {
				t.Errorf("Summary(%v) = %q, want %q", test.states, got, test.want)
			}
		})
	}
}

func TestUnreachableStripsTheSentinelPrefix(t *testing.T) {
	t.Parallel()

	target := domain.Target{Name: "example.com", Address: "https://example.com"}
	cause := fmt.Errorf("%w: dial tcp: connection refused", domain.ErrUnreachable)

	got := domain.Unreachable(target, cause)

	if got.Detail != "dial tcp: connection refused" {
		t.Errorf("Detail = %q, want the cause without the %q prefix", got.Detail, domain.ErrUnreachable)
	}
}
