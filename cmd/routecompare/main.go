package main

import (
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
		fmt.Fprintln(os.Stderr, "routecompare:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("routecompare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pre := flags.String("pre", "", "pre-change Junos XML file")
	post := flags.String("post", "", "post-change Junos XML file")
	vrf := flags.String("vrf", "ALL", "comma-separated routing tables, or ALL")
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
	render(stdout, "PRE-CHANGE ONLY", diff.BeforeOnly)
	render(stdout, "POST-CHANGE ONLY", diff.AfterOnly)
	return nil
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

func render(w io.Writer, title string, routes []routecompare.Route) {
	fmt.Fprintf(w, "\n%s (%d)\n", title, len(routes))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DESTINATION\tNEXT HOPS\tPROTOCOL\tTABLE")
	for _, route := range routes {
		hops := make([]string, len(route.NextHops))
		for i, hop := range route.NextHops {
			hops[i] = strings.Trim(strings.Join([]string{hop.To, hop.Via, hop.LocalInterface}, " "), " ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", route.Destination, strings.Join(hops, ", "), route.Protocol, route.Table)
	}
	_ = tw.Flush()
}
