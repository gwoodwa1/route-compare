package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	routecompare "github.com/gwoodwa1/route-compare"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(stdout.String()), routecompare.Version; got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
}

func TestRunRequiresBothSnapshots(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"-pre", "pre.xml"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "both -pre and -post") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSplitTables(t *testing.T) {
	got := splitTables(" inet.0, inet6.0, ")
	if len(got) != 2 || got[0] != "inet.0" || got[1] != "inet6.0" {
		t.Fatalf("unexpected tables: %#v", got)
	}
	if got := splitTables("ALL"); got != nil {
		t.Fatalf("ALL = %#v, want nil", got)
	}
}

func TestRenderJSON(t *testing.T) {
	diff := routecompare.Difference{
		BeforeCount:    2,
		AfterCount:     2,
		UnchangedCount: 1,
		Modified: []routecompare.RouteChange{{
			Before:        routecompare.Route{Destination: "192.0.2.0/24", Table: "inet.0", Preference: "5"},
			After:         routecompare.Route{Destination: "192.0.2.0/24", Table: "inet.0", Preference: "10"},
			ChangedFields: []string{"preference"},
		}},
	}
	result := buildReport(
		reportMetadata{GeneratedAt: "2026-09-02T12:00:00Z", ToolVersion: routecompare.Version},
		inputMetadata{Path: "before.xml", SHA256: strings.Repeat("a", 64)},
		inputMetadata{Path: "after.xml", SHA256: strings.Repeat("b", 64)},
		reportFilters{},
		diff,
	)
	var output bytes.Buffer
	if err := renderJSON(&output, result); err != nil {
		t.Fatal(err)
	}
	var decoded report
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Summary.Modified != 1 || len(decoded.Modified) != 1 || decoded.Added == nil || decoded.Removed == nil {
		t.Fatalf("unexpected report: %#v", decoded)
	}
	if decoded.Before.Path != "before.xml" || len(decoded.Before.SHA256) != 64 || decoded.Metadata.ToolVersion != routecompare.Version {
		t.Fatalf("unexpected metadata: %#v", decoded)
	}
}

func TestFailPolicies(t *testing.T) {
	diff := routecompare.Difference{
		BeforeOnly: []routecompare.Route{{Destination: "192.0.2.0/24"}},
		Modified:   []routecompare.RouteChange{{}},
	}
	if !matchesFailPolicy("any", diff) || !matchesFailPolicy("modified", diff) {
		t.Fatal("expected any and modified policies to match")
	}
	if matchesFailPolicy("added", diff) || matchesFailPolicy("removed", diff) || matchesFailPolicy("none", diff) {
		t.Fatal("unexpected policy match")
	}
	if validFailPolicy("surprise") {
		t.Fatal("unexpected valid fail policy")
	}
}

func TestFilterRoutes(t *testing.T) {
	prefixes, _, err := parsePrefixes("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	routes := []routecompare.Route{
		{Destination: "10.20.0.0/16", Protocol: "BGP"},
		{Destination: "10.30.0.0/16", Protocol: "OSPF"},
		{Destination: "192.0.2.0/24", Protocol: "BGP"},
	}
	got := filterRoutes(routes, []string{"bgp"}, prefixes)
	if len(got) != 1 || got[0].Destination != "10.20.0.0/16" {
		t.Fatalf("unexpected filtered routes: %#v", got)
	}
	if _, _, err := parsePrefixes("not-a-prefix"); err == nil {
		t.Fatal("expected invalid prefix error")
	}
}

func TestMarkdownAndHTMLReports(t *testing.T) {
	diff := routecompare.Difference{
		BeforeCount: 1,
		AfterCount:  1,
		Added:       []routecompare.Route{{Destination: "192.0.2.0/24", Table: "blue.inet.0", Protocol: "BGP"}},
	}
	result := buildReport(
		reportMetadata{GeneratedAt: "2026-09-02T12:00:00Z", ToolVersion: routecompare.Version, Device: "edge-01", ChangeID: "CHG-123"},
		inputMetadata{Path: "before.xml", SHA256: strings.Repeat("a", 64)},
		inputMetadata{Path: "after.xml", SHA256: strings.Repeat("b", 64)},
		reportFilters{},
		diff,
	)
	var markdown bytes.Buffer
	if err := renderMarkdown(&markdown, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown.String(), "# Route comparison report") || !strings.Contains(markdown.String(), "CHG-123") {
		t.Fatalf("unexpected Markdown report: %s", markdown.String())
	}
	var html bytes.Buffer
	if err := renderHTML(&html, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html.String(), "<!doctype html>") || !strings.Contains(html.String(), "edge-01") || !strings.Contains(html.String(), "192.0.2.0/24") {
		t.Fatalf("unexpected HTML report: %s", html.String())
	}
}

func TestJUnitReport(t *testing.T) {
	result := report{
		Metadata: reportMetadata{Device: "edge-01"},
		Summary:  reportSummary{Before: 10, After: 9, Removed: 1},
		Failed:   true,
	}
	var output bytes.Buffer
	if err := renderJUnit(&output, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `failures="1"`) || !strings.Contains(output.String(), `<failure message="route comparison failed"`) {
		t.Fatalf("unexpected JUnit report: %s", output.String())
	}
}

func TestRunRejectsMissingTable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-pre", "../../testdata/fixtures/pre_1.xml",
		"-post", "../../testdata/fixtures/post_1.xml",
		"-vrf", "missing.inet.0",
	}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWritesMarkdownFile(t *testing.T) {
	path := t.TempDir() + "/report.md"
	var stdout, stderr bytes.Buffer
	if err := run([]string{
		"-pre", "../../testdata/fixtures/pre_1.xml",
		"-post", "../../testdata/fixtures/post_1.xml",
		"-format", "markdown",
		"-output", path,
		"-device", "edge-01",
	}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || !strings.Contains(string(data), "edge-01") {
		t.Fatalf("unexpected report output: stdout=%q report=%q", stdout.String(), data)
	}
}

func TestLoadAndEvaluatePolicy(t *testing.T) {
	policy, err := loadPolicy("../../testdata/policies/maintenance.json")
	if err != nil {
		t.Fatal(err)
	}
	if policy.Name != "standard-maintenance" || policy.MaxModified == nil || *policy.MaxModified != 3 {
		t.Fatalf("unexpected policy: %#v", policy)
	}
	diff := routecompare.Difference{
		Modified: []routecompare.RouteChange{{
			Before: routecompare.Route{Destination: "192.0.2.0/24", Table: "inet.0"},
		}},
	}
	result, err := evaluatePolicy(policy, diff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || len(result.Violations) != 1 || !strings.Contains(result.Violations[0], "critical prefix") {
		t.Fatalf("unexpected policy evaluation: %#v", result)
	}
}

func TestRunUsesPolicyAndReturnsDifferenceError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-pre", "../../testdata/fixtures/pre_2.xml",
		"-post", "../../testdata/fixtures/post_2.xml",
		"-policy", "../../testdata/policies/maintenance.json",
		"-format", "json",
	}, &stdout, &stderr)
	var differenceErr differenceFoundError
	if !errors.As(err, &differenceErr) {
		t.Fatalf("expected policy difference error, got %v", err)
	}
	var result report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Policy == nil || result.Policy.Passed || result.Policy.Name != "standard-maintenance" {
		t.Fatalf("unexpected policy report: %#v", result.Policy)
	}
}

func TestRunBatchJSONAndJUnit(t *testing.T) {
	var jsonOutput, stderr bytes.Buffer
	err := run([]string{
		"-batch", "../../testdata/batch.json",
		"-format", "json",
	}, &jsonOutput, &stderr)
	var differenceErr differenceFoundError
	if !errors.As(err, &differenceErr) {
		t.Fatalf("expected batch difference error, got %v", err)
	}
	var batch batchReport
	if err := json.Unmarshal(jsonOutput.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Reports) != 2 || batch.Reports[0].Metadata.Device != "edge-01" || batch.Reports[1].Policy == nil {
		t.Fatalf("unexpected batch report: %#v", batch)
	}

	var junitOutput bytes.Buffer
	err = run([]string{
		"-batch", "../../testdata/batch.json",
		"-format", "junit",
	}, &junitOutput, &stderr)
	if !errors.As(err, &differenceErr) {
		t.Fatalf("expected batch difference error, got %v", err)
	}
	if !strings.Contains(junitOutput.String(), `tests="2"`) || !strings.Contains(junitOutput.String(), `failures="1"`) {
		t.Fatalf("unexpected batch JUnit: %s", junitOutput.String())
	}
}
