package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strings"
	"time"

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
	protocol := flags.String("protocol", "ALL", "comma-separated protocols, or ALL")
	prefix := flags.String("prefix", "ALL", "comma-separated covering IP prefixes, or ALL")
	changeType := flags.String("change-type", "ALL", "display added, removed, modified, or ALL")
	format := flags.String("format", "text", "output format: text, json, markdown, or html")
	output := flags.String("output", "", "write the report to a file instead of stdout")
	device := flags.String("device", "", "device name to include in report metadata")
	changeID := flags.String("change-id", "", "change or ticket identifier for report metadata")
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
	if !validOutputFormat(outputFormat) {
		return fmt.Errorf("unsupported format %q (use text, json, markdown, or html)", *format)
	}
	policy := strings.ToLower(strings.TrimSpace(*failOn))
	if !validFailPolicy(policy) {
		return fmt.Errorf("unsupported fail policy %q (use none, any, added, removed, or modified)", *failOn)
	}

	changeTypes := splitList(*changeType, true)
	if err := validateChangeTypes(changeTypes); err != nil {
		return err
	}
	prefixes, prefixNames, err := parsePrefixes(*prefix)
	if err != nil {
		return err
	}

	before, beforeInput, err := loadSnapshot(*pre)
	if err != nil {
		return err
	}
	after, afterInput, err := loadSnapshot(*post)
	if err != nil {
		return err
	}
	tables := splitTables(*vrf)
	if missing := before.MissingTables(tables...); len(missing) > 0 {
		return fmt.Errorf("routing table(s) not found in pre-change snapshot: %s", strings.Join(missing, ", "))
	}
	if missing := after.MissingTables(tables...); len(missing) > 0 {
		return fmt.Errorf("routing table(s) not found in post-change snapshot: %s", strings.Join(missing, ", "))
	}
	protocols := splitList(*protocol, false)
	beforeRoutes := filterRoutes(before.Routes(tables...), protocols, prefixes)
	afterRoutes := filterRoutes(after.Routes(tables...), protocols, prefixes)
	diff := (routecompare.Comparator{}).Compare(beforeRoutes, afterRoutes)
	report := buildReport(
		reportMetadata{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			ToolVersion: routecompare.Version,
			Device:      strings.TrimSpace(*device),
			ChangeID:    strings.TrimSpace(*changeID),
		},
		beforeInput,
		afterInput,
		reportFilters{
			Tables:      nonNilStrings(tables),
			Protocols:   nonNilStrings(protocols),
			Prefixes:    nonNilStrings(prefixNames),
			ChangeTypes: nonNilStrings(changeTypes),
		},
		diff,
	)
	reportWriter := stdout
	var outputFile *os.File
	if *output != "" {
		outputFile, err = os.Create(*output)
		if err != nil {
			return fmt.Errorf("create report %q: %w", *output, err)
		}
		defer outputFile.Close()
		reportWriter = outputFile
	}
	if err := writeReport(reportWriter, outputFormat, report); err != nil {
		return err
	}
	if matchesFailPolicy(policy, diff) {
		return differenceFoundError{policy: policy}
	}
	return nil
}

func validOutputFormat(format string) bool {
	switch format {
	case "text", "json", "markdown", "html":
		return true
	default:
		return false
	}
}

func writeReport(w io.Writer, format string, report report) error {
	switch format {
	case "text":
		renderText(w, report)
		return nil
	case "json":
		return renderJSON(w, report)
	case "markdown":
		return renderMarkdown(w, report)
	case "html":
		return renderHTML(w, report)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func loadSnapshot(path string) (*routecompare.Snapshot, inputMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, inputMetadata{}, fmt.Errorf("read route snapshot %q: %w", path, err)
	}
	snapshot, err := routecompare.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, inputMetadata{}, fmt.Errorf("%s: %w", path, err)
	}
	hash := sha256.Sum256(data)
	return snapshot, inputMetadata{Path: path, SHA256: fmt.Sprintf("%x", hash)}, nil
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
	return splitList(value, false)
}

func splitList(value string, lower bool) []string {
	if strings.EqualFold(strings.TrimSpace(value), "ALL") || strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := parts[:0]
	seen := make(map[string]struct{})
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, value)
	}
	return values
}

func validateChangeTypes(changeTypes []string) error {
	for _, changeType := range changeTypes {
		switch changeType {
		case "added", "removed", "modified":
		default:
			return fmt.Errorf("unsupported change type %q (use added, removed, modified, or ALL)", changeType)
		}
	}
	return nil
}

func parsePrefixes(value string) ([]netip.Prefix, []string, error) {
	names := splitList(value, false)
	prefixes := make([]netip.Prefix, len(names))
	for i, name := range names {
		prefix, err := netip.ParsePrefix(name)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid prefix %q: %w", name, err)
		}
		prefixes[i] = prefix.Masked()
		names[i] = prefixes[i].String()
	}
	return prefixes, names, nil
}

func filterRoutes(routes []routecompare.Route, protocols []string, prefixes []netip.Prefix) []routecompare.Route {
	protocolSet := make(map[string]struct{}, len(protocols))
	for _, protocol := range protocols {
		protocolSet[strings.ToLower(protocol)] = struct{}{}
	}
	filtered := make([]routecompare.Route, 0, len(routes))
	for _, route := range routes {
		if len(protocolSet) > 0 {
			if _, ok := protocolSet[strings.ToLower(route.Protocol)]; !ok {
				continue
			}
		}
		if len(prefixes) > 0 && !matchesPrefix(route.Destination, prefixes) {
			continue
		}
		filtered = append(filtered, route)
	}
	return filtered
}

func matchesPrefix(destination string, filters []netip.Prefix) bool {
	routePrefix, err := netip.ParsePrefix(destination)
	if err != nil {
		return false
	}
	for _, filter := range filters {
		if filter.Addr().BitLen() == routePrefix.Addr().BitLen() && filter.Bits() <= routePrefix.Bits() && filter.Contains(routePrefix.Addr()) {
			return true
		}
	}
	return false
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
