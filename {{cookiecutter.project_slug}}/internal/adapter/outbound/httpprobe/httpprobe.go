// Package httpprobe is the driven adapter that talks to real external systems
// over HTTP. It is the only place in the application that knows HTTP exists.
package httpprobe

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/core/port"
)

// DefaultTimeout bounds a single probe.
const DefaultTimeout = 5 * time.Second

// Adapter implements port.Prober against a live HTTP endpoint.
type Adapter struct {
	client    *http.Client
	clock     port.Clock
	userAgent string
}

// Option customises the adapter.
type Option func(*Adapter)

// WithClient supplies a pre-configured HTTP client, for callers that need
// custom transports, proxies or TLS settings.
func WithClient(c *http.Client) Option {
	return func(a *Adapter) {
		if c != nil {
			a.client = c
		}
	}
}

// WithTimeout bounds each request.
func WithTimeout(d time.Duration) Option {
	return func(a *Adapter) {
		if d > 0 {
			a.client.Timeout = d
		}
	}
}

// WithClock replaces the clock used to measure latency.
func WithClock(c port.Clock) Option {
	return func(a *Adapter) {
		if c != nil {
			a.clock = c
		}
	}
}

// WithUserAgent sets the User-Agent header sent to external systems.
func WithUserAgent(ua string) Option {
	return func(a *Adapter) {
		if ua != "" {
			a.userAgent = ua
		}
	}
}

// New builds an HTTP prober.
func New(opts ...Option) *Adapter {
	a := &Adapter{
		client:    &http.Client{Timeout: DefaultTimeout},
		clock:     systemClock{},
		userAgent: "{{ cookiecutter.binary_name }}",
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Probe performs one GET and reports the transport facts. Every failure is
// wrapped in domain.ErrUnreachable so the core stays free of HTTP semantics.
func (a *Adapter) Probe(ctx context.Context, target domain.Target) (domain.Probe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.Address, http.NoBody)
	if err != nil {
		return domain.Probe{}, fmt.Errorf("%w: %w", domain.ErrUnreachable, err)
	}
	req.Header.Set("User-Agent", a.userAgent)

	start := a.clock.Now()
	resp, err := a.client.Do(req)
	if err != nil {
		return domain.Probe{}, fmt.Errorf("%w: %w", domain.ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	return domain.Probe{
		StatusCode: resp.StatusCode,
		Latency:    a.clock.Since(start),
	}, nil
}

// systemClock is the real implementation of port.Clock.
type systemClock struct{}

func (systemClock) Now() time.Time                  { return time.Now() }
func (systemClock) Since(t time.Time) time.Duration { return time.Since(t) }

// SystemClock returns the wall-clock implementation of port.Clock.
func SystemClock() port.Clock { return systemClock{} }

var _ port.Prober = (*Adapter)(nil)
