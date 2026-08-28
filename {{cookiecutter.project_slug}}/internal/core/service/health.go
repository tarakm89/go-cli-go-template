// Package service implements the inbound ports. It orchestrates domain rules
// and outbound ports and is the only place where a use case lives.
//
// It depends on domain and port. It must never import an adapter — the
// depguard rule in .golangci.yml enforces that.
package service

import (
	"context"
	"sync"

	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/core/port"
)

// Health is the use case behind the `check` command.
type Health struct {
	prober      port.Prober
	thresholds  domain.Thresholds
	concurrency int
}

// Option customises a Health service.
type Option func(*Health)

// WithThresholds overrides the latency budget used to grade responses.
func WithThresholds(t domain.Thresholds) Option {
	return func(h *Health) { h.thresholds = t }
}

// WithConcurrency caps how many targets are probed at once. Values below one
// are ignored.
func WithConcurrency(n int) Option {
	return func(h *Health) {
		if n > 0 {
			h.concurrency = n
		}
	}
}

// NewHealth wires the use case to an outbound prober.
func NewHealth(prober port.Prober, opts ...Option) *Health {
	h := &Health{
		prober:      prober,
		thresholds:  domain.DefaultThresholds(),
		concurrency: 8,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Check inspects a single target.
func (h *Health) Check(ctx context.Context, target domain.Target) domain.Health {
	probe, err := h.prober.Probe(ctx, target)
	if err != nil {
		return domain.Unreachable(target, err)
	}
	return h.thresholds.Evaluate(target, probe)
}

// CheckAll inspects every target concurrently, preserving input order. A
// cancelled context is not an error either: the targets that did not get a
// verdict come back as unreachable, which is what the user needs to see.
func (h *Health) CheckAll(ctx context.Context, targets []domain.Target) []domain.Health {
	results := make([]domain.Health, len(targets))

	var wg sync.WaitGroup
	slots := make(chan struct{}, h.concurrency)

	for i, target := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			results[i] = h.Check(ctx, target)
		}()
	}
	wg.Wait()

	return results
}

// Compile-time proof that the service satisfies the inbound port.
var _ port.HealthChecker = (*Health)(nil)
