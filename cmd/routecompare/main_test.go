package main

import (
	"bytes"
	"encoding/json"
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
	var output bytes.Buffer
	if err := renderJSON(&output, "before.xml", "after.xml", diff); err != nil {
		t.Fatal(err)
	}
	var report jsonReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Modified != 1 || len(report.Modified) != 1 || report.Added == nil || report.Removed == nil {
		t.Fatalf("unexpected report: %#v", report)
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
