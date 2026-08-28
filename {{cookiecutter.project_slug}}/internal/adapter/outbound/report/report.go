// Package report contains the driven adapters that present results. Two
// adapters sit behind the single port.Reporter interface, which is the
// cheapest possible demonstration of why the port exists.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"{{ cookiecutter.module_path }}/internal/core/domain"
	"{{ cookiecutter.module_path }}/internal/core/port"
)

// Format selects a reporter implementation.
type Format string

// Supported output formats.
const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Formats lists every supported format, for flag help and validation.
func Formats() []string { return []string{string(FormatText), string(FormatJSON)} }

// New returns the reporter for the named format.
func New(format string, w io.Writer) (port.Reporter, error) {
	switch Format(strings.ToLower(strings.TrimSpace(format))) {
	case FormatText:
		return &Text{out: w}, nil
	case FormatJSON:
		return &JSON{out: w}, nil
	default:
		return nil, fmt.Errorf("unknown output format %q (want one of %s)", format, strings.Join(Formats(), ", "))
	}
}

// Text renders a human readable table.
type Text struct{ out io.Writer }

// Report implements port.Reporter.
func (t *Text) Report(checks []domain.Health) error {
	if len(checks) == 0 {
		_, err := fmt.Fprintln(t.out, "no targets checked")
		return err
	}

	tw := tabwriter.NewWriter(t.out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TARGET\tSTATE\tLATENCY\tDETAIL"); err != nil {
		return err
	}
	for _, check := range checks {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			check.Target.Name,
			check.State,
			check.Latency.Round(time.Millisecond),
			check.Detail,
		); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(t.out, "\nsummary: %s\n", domain.Summary(checks))
	return err
}

// JSON renders a machine readable document, so the CLI composes with jq and
// with whatever runs it in CI.
type JSON struct{ out io.Writer }

type jsonCheck struct {
	Target    string `json:"target"`
	Address   string `json:"address"`
	State     string `json:"state"`
	LatencyMS int64  `json:"latency_ms"`
	Detail    string `json:"detail"`
}

type jsonDocument struct {
	Summary string      `json:"summary"`
	Checks  []jsonCheck `json:"checks"`
}

// Report implements port.Reporter.
func (j *JSON) Report(checks []domain.Health) error {
	doc := jsonDocument{
		Summary: domain.Summary(checks).String(),
		Checks:  make([]jsonCheck, 0, len(checks)),
	}
	for _, check := range checks {
		doc.Checks = append(doc.Checks, jsonCheck{
			Target:    check.Target.Name,
			Address:   check.Target.Address,
			State:     check.State.String(),
			LatencyMS: check.Latency.Milliseconds(),
			Detail:    check.Detail,
		})
	}

	enc := json.NewEncoder(j.out)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

var (
	_ port.Reporter = (*Text)(nil)
	_ port.Reporter = (*JSON)(nil)
)
