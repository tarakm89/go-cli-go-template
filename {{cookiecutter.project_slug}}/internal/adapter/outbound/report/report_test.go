package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"{{ cookiecutter.module_path }}/internal/adapter/outbound/report"
	"{{ cookiecutter.module_path }}/internal/core/domain"
)

func sample() []domain.Health {
	return []domain.Health{
		{
			Target:  domain.Target{Name: "a.example.com", Address: "https://a.example.com"},
			State:   domain.StateUp,
			Latency: 12 * time.Millisecond,
			Detail:  "status 200",
		},
		{
			Target:  domain.Target{Name: "b.example.com", Address: "https://b.example.com"},
			State:   domain.StateDown,
			Latency: 0,
			Detail:  "connection refused",
		},
	}
}

func TestNewUnknownFormat(t *testing.T) {
	t.Parallel()

	if _, err := report.New("yaml", &bytes.Buffer{}); err == nil {
		t.Fatal("an unknown format must be rejected")
	}
}

func TestTextReport(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter, err := report.New("text", &out)
	if err != nil {
		t.Fatalf("report.New: %v", err)
	}
	if err := reporter.Report(sample()); err != nil {
		t.Fatalf("Report: %v", err)
	}

	got := out.String()
	for _, want := range []string{"TARGET", "a.example.com", "b.example.com", "connection refused", "summary: down"} {
		if !strings.Contains(got, want) {
			t.Errorf("text report is missing %q\n---\n%s", want, got)
		}
	}
}

func TestTextReportEmpty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter, _ := report.New("text", &out)
	if err := reporter.Report(nil); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if !strings.Contains(out.String(), "no targets") {
		t.Errorf("an empty report should say so, got %q", out.String())
	}
}

func TestJSONReportIsMachineReadable(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter, err := report.New("json", &out)
	if err != nil {
		t.Fatalf("report.New: %v", err)
	}
	if err := reporter.Report(sample()); err != nil {
		t.Fatalf("Report: %v", err)
	}

	var doc struct {
		Summary string `json:"summary"`
		Checks  []struct {
			Target    string `json:"target"`
			Address   string `json:"address"`
			State     string `json:"state"`
			LatencyMS int64  `json:"latency_ms"`
			Detail    string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}

	if doc.Summary != "down" {
		t.Errorf("summary = %q, want %q", doc.Summary, "down")
	}
	if len(doc.Checks) != 2 {
		t.Fatalf("got %d checks, want 2", len(doc.Checks))
	}
	if doc.Checks[0].LatencyMS != 12 {
		t.Errorf("latency_ms = %d, want 12", doc.Checks[0].LatencyMS)
	}
	if doc.Checks[1].State != "down" {
		t.Errorf("checks[1].state = %q, want %q", doc.Checks[1].State, "down")
	}
}

func TestJSONReportEmptyIsAnArrayNotNull(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter, _ := report.New("json", &out)
	if err := reporter.Report(nil); err != nil {
		t.Fatalf("Report: %v", err)
	}

	// `jq '.checks | length'` in a pipeline must not fall over on null.
	if !strings.Contains(out.String(), `"checks": []`) {
		t.Errorf("empty checks should serialise as [], got %s", out.String())
	}
}
