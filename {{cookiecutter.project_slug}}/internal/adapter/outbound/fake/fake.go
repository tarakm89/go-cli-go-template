// Package fake holds in-memory implementations of every outbound port.
//
// They are shipped as ordinary adapters rather than hidden in _test.go files
// on purpose: the functional suite in test/functional wires the real
// application — the real cobra commands, the real use case, the real
// reporters — and swaps only the adapters that would otherwise reach an
// external system. That is the property the hexagon buys you, and it is worth
// exercising from a package that any test in the module can import.
package fake

import (
	"context"
	"fmt"
	"sync"
	"time"

	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/core/port"
)

// Response is what a Prober should return for one address.
type Response struct {
	// StatusCode is reported when Err is nil.
	StatusCode int
	// Latency is reported as the observed round trip.
	Latency time.Duration
	// Err, when set, is returned instead of a probe. Wrap
	// domain.ErrUnreachable to imitate a real transport failure.
	Err error
	// Delay makes Probe actually block, which is how a test exercises
	// timeouts and cancellation.
	Delay time.Duration
}

// Prober is an in-memory port.Prober. The zero value is usable and answers
// every address with 200 OK.
type Prober struct {
	mu        sync.Mutex
	responses map[string]Response
	fallback  Response
	calls     []domain.Target
}

// NewProber returns a prober that answers 200 OK for anything it has not been
// told about.
func NewProber() *Prober {
	return &Prober{
		responses: make(map[string]Response),
		fallback:  Response{StatusCode: 200, Latency: 10 * time.Millisecond},
	}
}

// With programs the response for one address and returns the prober, so
// several can be chained in a test setup.
func (p *Prober) With(address string, response Response) *Prober {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.responses == nil {
		p.responses = make(map[string]Response)
	}
	p.responses[address] = response
	return p
}

// WithFallback changes the answer given for unprogrammed addresses.
func (p *Prober) WithFallback(response Response) *Prober {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fallback = response
	return p
}

// WithFailure is shorthand for programming an unreachable address.
func (p *Prober) WithFailure(address, reason string) *Prober {
	return p.With(address, Response{
		Err: fmt.Errorf("%w: %s", domain.ErrUnreachable, reason),
	})
}

// Probe implements port.Prober.
func (p *Prober) Probe(ctx context.Context, target domain.Target) (domain.Probe, error) {
	p.mu.Lock()
	response, ok := p.responses[target.Address]
	if !ok {
		response = p.fallback
	}
	p.calls = append(p.calls, target)
	p.mu.Unlock()

	if response.Delay > 0 {
		timer := time.NewTimer(response.Delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return domain.Probe{}, fmt.Errorf("%w: %w", domain.ErrUnreachable, ctx.Err())
		}
	}

	if response.Err != nil {
		return domain.Probe{}, response.Err
	}

	return domain.Probe{StatusCode: response.StatusCode, Latency: response.Latency}, nil
}

// Calls returns the targets probed so far, in the order they were seen.
func (p *Prober) Calls() []domain.Target {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]domain.Target(nil), p.calls...)
}

// CallCount returns how many probes were made.
func (p *Prober) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

// Reset forgets recorded calls, keeping the programmed responses.
func (p *Prober) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = nil
}

// Clock is a deterministic port.Clock. Every reading advances by Step, so a
// test can assert on an exact latency instead of a range.
type Clock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

// NewClock returns a clock starting at a fixed instant and advancing by step
// on every call to Since.
func NewClock(start time.Time, step time.Duration) *Clock {
	return &Clock{now: start, step: step}
}

// Now implements port.Clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Since implements port.Clock, advancing the clock by one step.
func (c *Clock) Since(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(c.step)
	return c.now.Sub(t)
}

// Advance moves the clock forward by d.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// Reporter is an in-memory port.Reporter that keeps what it was asked to
// render, so a test can assert on verdicts rather than on formatting.
type Reporter struct {
	mu       sync.Mutex
	reported [][]domain.Health
	err      error
}

// NewReporter returns an empty reporter.
func NewReporter() *Reporter { return &Reporter{} }

// WithError makes Report fail, to exercise the error path.
func (r *Reporter) WithError(err error) *Reporter {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
	return r
}

// Report implements port.Reporter.
func (r *Reporter) Report(checks []domain.Health) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reported = append(r.reported, append([]domain.Health(nil), checks...))
	return r.err
}

// Reports returns every batch handed to Report.
func (r *Reporter) Reports() [][]domain.Health {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]domain.Health(nil), r.reported...)
}

// Last returns the most recent batch, or nil if Report was never called.
func (r *Reporter) Last() []domain.Health {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reported) == 0 {
		return nil
	}
	return r.reported[len(r.reported)-1]
}

// Record is one captured log line.
type Record struct {
	Level   string
	Message string
	Attrs   []any
}

// Logger is an in-memory port.Logger.
type Logger struct {
	mu      sync.Mutex
	records []Record
}

// NewLogger returns an empty logger.
func NewLogger() *Logger { return &Logger{} }

// Debug implements port.Logger.
func (l *Logger) Debug(_ context.Context, msg string, attrs ...any) { l.add("debug", msg, attrs) }

// Info implements port.Logger.
func (l *Logger) Info(_ context.Context, msg string, attrs ...any) { l.add("info", msg, attrs) }

// Warn implements port.Logger.
func (l *Logger) Warn(_ context.Context, msg string, attrs ...any) { l.add("warn", msg, attrs) }

// Error implements port.Logger.
func (l *Logger) Error(_ context.Context, msg string, attrs ...any) { l.add("error", msg, attrs) }

func (l *Logger) add(level, msg string, attrs []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, Record{Level: level, Message: msg, Attrs: attrs})
}

// Records returns everything logged so far.
func (l *Logger) Records() []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Record(nil), l.records...)
}

// Messages returns just the messages, which is usually what an assertion wants.
func (l *Logger) Messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	messages := make([]string, 0, len(l.records))
	for _, record := range l.records {
		messages = append(messages, record.Message)
	}
	return messages
}

var (
	_ port.Prober   = (*Prober)(nil)
	_ port.Clock    = (*Clock)(nil)
	_ port.Reporter = (*Reporter)(nil)
	_ port.Logger   = (*Logger)(nil)
)
