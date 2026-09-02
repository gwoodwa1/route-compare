package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	routecompare "github.com/gwoodwa1/route-compare"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var differenceErr differenceFoundError
		if errors.As(err, &differenceErr) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "routecompare:", err)
		os.Exit(1)
	}
}

type differenceFoundError struct{ policy string }

func (e differenceFoundError) Error() string { return "differences matched fail policy " + e.policy }

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("routecompare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pre := flags.String("pre", "", "pre-change Junos XML file")
	post := flags.String("post", "", "post-change Junos XML file")
	vrf := flags.String("vrf", "ALL", "comma-separated routing tables, or ALL")
	format := flags.String("format", "text", "output format: text or json")
	failOn := flags.String("fail-on", "none", "return exit code 2 on: none, any, added, removed, or modified")
	version := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *version {
		fmt.Fprintln(stdout, routecompare.Version)
		return nil
	}
	if *pre == "" || *post == "" {
		return fmt.Errorf("both -pre and -post are required")
	}
	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	if outputFormat != "text" && outputFormat != "json" {
		return fmt.Errorf("unsupported format %q (use text or json)", *format)
	}
	policy := strings.ToLower(strings.TrimSpace(*failOn))
	if !validFailPolicy(policy) {
		return fmt.Errorf("unsupported fail policy %q (use none, any, added, removed, or modified)", *failOn)
	}

	before, err := routecompare.ParseFile(*pre)
	if err != nil {
		return err
	}
	after, err := routecompare.ParseFile(*post)
	if err != nil {
		return err
	}
	tables := splitTables(*vrf)
	diff := (routecompare.Comparator{}).Compare(before.Routes(tables...), after.Routes(tables...))
	if outputFormat == "json" {
		if err := renderJSON(stdout, *pre, *post, diff); err != nil {
			return err
		}
	} else {
		renderText(stdout, *pre, *post, diff)
	}
	if matchesFailPolicy(policy, diff) {
		return differenceFoundError{policy: policy}
	}
	return nil
}

func validFailPolicy(policy string) bool {
	switch policy {
	case "none", "any", "added", "removed", "modified":
		return true
	default:
		return false
	}
}

func matchesFailPolicy(policy string, diff routecompare.Difference) bool {
	switch policy {
	case "any":
		return !diff.Empty()
	case "added":
		return len(diff.Added) > 0
	case "removed":
		return len(diff.Removed) > 0
	case "modified":
		return len(diff.Modified) > 0
	default:
		return false
	}
}

func splitTables(value string) []string {
	if strings.EqualFold(strings.TrimSpace(value), "ALL") {
		return nil
	}
	parts := strings.Split(value, ",")
	tables := parts[:0]
	for _, part := range parts {
		if table := strings.TrimSpace(part); table != "" {
			tables = append(tables, table)
		}
	}
	return tables
}

func renderText(w io.Writer, pre, post string, diff routecompare.Difference) {
	fmt.Fprintln(w, "ROUTE COMPARISON")
	fmt.Fprintf(w, "Before: %s\n", pre)
	fmt.Fprintf(w, "After:  %s\n", post)
	fmt.Fprintln(w, "\nSUMMARY")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "Routes before\t%d\n", diff.BeforeCount)
	fmt.Fprintf(tw, "Routes after\t%d\n", diff.AfterCount)
	fmt.Fprintf(tw, "Unchanged\t%d\n", diff.UnchangedCount)
	fmt.Fprintf(tw, "Added\t%d\n", len(diff.Added))
	fmt.Fprintf(tw, "Removed\t%d\n", len(diff.Removed))
	fmt.Fprintf(tw, "Modified\t%d\n", len(diff.Modified))
	_ = tw.Flush()

	renderRoutes(w, "ADDED", diff.Added)
	renderRoutes(w, "REMOVED", diff.Removed)
	renderModified(w, diff.Modified)
}

func renderRoutes(w io.Writer, title string, routes []routecompare.Route) {
	fmt.Fprintf(w, "\n%s (%d)\n", title, len(routes))
	if len(routes) == 0 {
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DESTINATION\tNEXT HOPS\tPROTOCOL\tPREFERENCE\tTABLE")
	for _, route := range routes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", route.Destination, formatNextHops(route.NextHops), route.Protocol, route.Preference, route.Table)
	}
	_ = tw.Flush()
}

func renderModified(w io.Writer, changes []routecompare.RouteChange) {
	fmt.Fprintf(w, "\nMODIFIED (%d)\n", len(changes))
	for _, change := range changes {
		fmt.Fprintf(w, "%s  %s  %s\n", change.Before.Destination, change.Before.Table, strings.Join(change.ChangedFields, ", "))
		fmt.Fprintf(w, "  before: protocol=%s preference=%s next-hops=%s\n", change.Before.Protocol, change.Before.Preference, formatNextHops(change.Before.NextHops))
		fmt.Fprintf(w, "  after:  protocol=%s preference=%s next-hops=%s\n", change.After.Protocol, change.After.Preference, formatNextHops(change.After.NextHops))
	}
}

func formatNextHops(nextHops []routecompare.NextHop) string {
	hops := make([]string, len(nextHops))
	for i, hop := range nextHops {
		hops[i] = strings.Trim(strings.Join([]string{hop.To, hop.Via, hop.LocalInterface}, " "), " ")
	}
	return strings.Join(hops, ", ")
}

type jsonReport struct {
	Before   string                     `json:"before"`
	After    string                     `json:"after"`
	Summary  jsonSummary                `json:"summary"`
	Added    []routecompare.Route       `json:"added"`
	Removed  []routecompare.Route       `json:"removed"`
	Modified []routecompare.RouteChange `json:"modified"`
}

type jsonSummary struct {
	Before    int `json:"before"`
	After     int `json:"after"`
	Unchanged int `json:"unchanged"`
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Modified  int `json:"modified"`
}

func renderJSON(w io.Writer, pre, post string, diff routecompare.Difference) error {
	report := jsonReport{
		Before: pre,
		After:  post,
		Summary: jsonSummary{
			Before:    diff.BeforeCount,
			After:     diff.AfterCount,
			Unchanged: diff.UnchangedCount,
			Added:     len(diff.Added),
			Removed:   len(diff.Removed),
			Modified:  len(diff.Modified),
		},
		Added:    nonNilRoutes(diff.Added),
		Removed:  nonNilRoutes(diff.Removed),
		Modified: nonNilChanges(diff.Modified),
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}
	return nil
}

func nonNilRoutes(routes []routecompare.Route) []routecompare.Route {
	if routes == nil {
		return []routecompare.Route{}
	}
	return routes
}

func nonNilChanges(changes []routecompare.RouteChange) []routecompare.RouteChange {
	if changes == nil {
		return []routecompare.RouteChange{}
	}
	return changes
}
