package main

import (
	"bytes"
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
