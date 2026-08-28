package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"{{ cookiecutter.module_path }}/internal/core/domain"
)

// failOn names the worst state a run may reach before the exit code turns
// non-zero. A pipeline usually wants `down`; a canary job might want
// `degraded`; a dashboard-only job wants `never`.
type failOn string

const (
	failOnNever    failOn = "never"
	failOnDegraded failOn = "degraded"
	failOnDown     failOn = "down"
)

func failOnValues() []string {
	return []string{string(failOnNever), string(failOnDegraded), string(failOnDown)}
}

func parseFailOn(raw string) (failOn, error) {
	switch f := failOn(strings.ToLower(strings.TrimSpace(raw))); f {
	case failOnNever, failOnDegraded, failOnDown:
		return f, nil
	default:
		return "", fmt.Errorf("unknown --fail-on value %q (want one of %s)", raw, joinWords(failOnValues()))
	}
}

// breached reports whether summary is bad enough to fail the run.
func (f failOn) breached(summary domain.State) bool {
	switch f {
	case failOnNever:
		return false
	case failOnDegraded:
		return summary != domain.StateUp
	case failOnDown:
		return summary == domain.StateDown
	default:
		return false
	}
}

func (a *app) newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check TARGET [TARGET...]",
		Short: "Check the health of one or more external systems",
		Long: "Probes every TARGET concurrently and reports a verdict for each.\n\n" +
			"A target is an http or https URL. The exit code is governed by\n" +
			"--fail-on, so this command can gate a pipeline step directly.",
		Example: "  {{ cookiecutter.binary_name }} check https://api.example.com/healthz\n" +
			"  {{ cookiecutter.binary_name }} check -o json https://a.example.com https://b.example.com\n" +
			"  {{ cookiecutter.binary_name }} check --fail-on degraded --degraded-after 200ms https://a.example.com",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE:         a.runCheck,
	}
	return cmd
}

func (a *app) runCheck(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	threshold, err := parseFailOn(a.failOn)
	if err != nil {
		return err
	}

	reporter, err := a.reporterFn()
	if err != nil {
		return err
	}

	targets, err := parseTargets(args)
	if err != nil {
		return err
	}

	a.logger.Info(ctx, "starting health check",
		"targets", len(targets),
		"concurrency", a.concurrency,
		"timeout", a.timeout.String(),
	)

	results := a.checker.CheckAll(ctx, targets)

	if err := reporter.Report(results); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	summary := domain.Summary(results)
	a.logger.Info(ctx, "health check finished", "summary", summary.String())

	if threshold.breached(summary) {
		// The command did its job; the world is unhealthy. Report that
		// through the exit code without printing a Go error on top of the
		// report the user just read.
		return &exitError{code: ExitUnhealthy, summary: summary}
	}

	return nil
}

// parseTargets validates every argument before probing any of them, so a typo
// in the last URL does not cost a round of network calls.
func parseTargets(args []string) ([]domain.Target, error) {
	targets := make([]domain.Target, 0, len(args))
	for _, arg := range args {
		target, err := domain.NewTarget(arg)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// exitError carries an exit code out of a command without a message: the
// report has already told the user everything they need.
type exitError struct {
	code    int
	summary domain.State
}

func (e *exitError) Error() string {
	return fmt.Sprintf("health check reported %s", e.summary)
}

// ExitCode implements the interface Run uses to map an error to a status.
func (e *exitError) ExitCode() int { return e.code }

// Silent reports that this error was already communicated to the user.
func (e *exitError) Silent() bool { return true }
