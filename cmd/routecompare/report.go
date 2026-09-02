package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"text/tabwriter"

	routecompare "github.com/gwoodwa1/route-compare"
)

type report struct {
	Metadata reportMetadata             `json:"metadata"`
	Before   inputMetadata              `json:"before"`
	After    inputMetadata              `json:"after"`
	Filters  reportFilters              `json:"filters"`
	Summary  reportSummary              `json:"summary"`
	Added    []routecompare.Route       `json:"added"`
	Removed  []routecompare.Route       `json:"removed"`
	Modified []routecompare.RouteChange `json:"modified"`
}

type reportMetadata struct {
	GeneratedAt string `json:"generated_at"`
	ToolVersion string `json:"tool_version"`
	Device      string `json:"device,omitempty"`
	ChangeID    string `json:"change_id,omitempty"`
}

type inputMetadata struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type reportFilters struct {
	Tables      []string `json:"tables"`
	Protocols   []string `json:"protocols"`
	Prefixes    []string `json:"prefixes"`
	ChangeTypes []string `json:"change_types"`
}

type reportSummary struct {
	Before    int `json:"before"`
	After     int `json:"after"`
	Unchanged int `json:"unchanged"`
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Modified  int `json:"modified"`
}

func buildReport(metadata reportMetadata, before, after inputMetadata, filters reportFilters, diff routecompare.Difference) report {
	result := report{
		Metadata: metadata,
		Before:   before,
		After:    after,
		Filters:  filters,
		Summary: reportSummary{
			Before:    diff.BeforeCount,
			After:     diff.AfterCount,
			Unchanged: diff.UnchangedCount,
			Added:     len(diff.Added),
			Removed:   len(diff.Removed),
			Modified:  len(diff.Modified),
		},
		Added:    []routecompare.Route{},
		Removed:  []routecompare.Route{},
		Modified: []routecompare.RouteChange{},
	}
	if includesChangeType(filters.ChangeTypes, "added") {
		result.Added = append(result.Added, diff.Added...)
	}
	if includesChangeType(filters.ChangeTypes, "removed") {
		result.Removed = append(result.Removed, diff.Removed...)
	}
	if includesChangeType(filters.ChangeTypes, "modified") {
		result.Modified = append(result.Modified, diff.Modified...)
	}
	return result
}

func includesChangeType(selected []string, changeType string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, candidate := range selected {
		if candidate == changeType {
			return true
		}
	}
	return false
}

func renderText(w io.Writer, result report) {
	fmt.Fprintln(w, "ROUTE COMPARISON")
	fmt.Fprintf(w, "Before:    %s\n", result.Before.Path)
	fmt.Fprintf(w, "After:     %s\n", result.After.Path)
	fmt.Fprintf(w, "Generated: %s\n", result.Metadata.GeneratedAt)
	if result.Metadata.Device != "" {
		fmt.Fprintf(w, "Device:    %s\n", result.Metadata.Device)
	}
	if result.Metadata.ChangeID != "" {
		fmt.Fprintf(w, "Change ID: %s\n", result.Metadata.ChangeID)
	}
	fmt.Fprintln(w, "\nSUMMARY")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "Routes before\t%d\n", result.Summary.Before)
	fmt.Fprintf(tw, "Routes after\t%d\n", result.Summary.After)
	fmt.Fprintf(tw, "Unchanged\t%d\n", result.Summary.Unchanged)
	fmt.Fprintf(tw, "Added\t%d\n", result.Summary.Added)
	fmt.Fprintf(tw, "Removed\t%d\n", result.Summary.Removed)
	fmt.Fprintf(tw, "Modified\t%d\n", result.Summary.Modified)
	_ = tw.Flush()

	renderTextRoutes(w, "ADDED", result.Added)
	renderTextRoutes(w, "REMOVED", result.Removed)
	renderTextModified(w, result.Modified)
}

func renderTextRoutes(w io.Writer, title string, routes []routecompare.Route) {
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

func renderTextModified(w io.Writer, changes []routecompare.RouteChange) {
	fmt.Fprintf(w, "\nMODIFIED (%d)\n", len(changes))
	for _, change := range changes {
		fmt.Fprintf(w, "%s  %s  %s\n", change.Before.Destination, change.Before.Table, strings.Join(change.ChangedFields, ", "))
		fmt.Fprintf(w, "  before: protocol=%s preference=%s next-hops=%s\n", change.Before.Protocol, change.Before.Preference, formatNextHops(change.Before.NextHops))
		fmt.Fprintf(w, "  after:  protocol=%s preference=%s next-hops=%s\n", change.After.Protocol, change.After.Preference, formatNextHops(change.After.NextHops))
	}
}

func renderJSON(w io.Writer, result report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}
	return nil
}

func renderMarkdown(w io.Writer, result report) error {
	status := "PASS"
	if result.Summary.Added+result.Summary.Removed+result.Summary.Modified > 0 {
		status = "CHANGES DETECTED"
	}
	fmt.Fprintln(w, "# Route comparison report")
	fmt.Fprintf(w, "\n**Result:** %s  \n", status)
	fmt.Fprintf(w, "**Before:** `%s`  \n", markdownEscape(result.Before.Path))
	fmt.Fprintf(w, "**After:** `%s`  \n", markdownEscape(result.After.Path))
	fmt.Fprintf(w, "**Generated:** %s  \n", result.Metadata.GeneratedAt)
	if result.Metadata.Device != "" {
		fmt.Fprintf(w, "**Device:** %s  \n", markdownEscape(result.Metadata.Device))
	}
	if result.Metadata.ChangeID != "" {
		fmt.Fprintf(w, "**Change ID:** %s  \n", markdownEscape(result.Metadata.ChangeID))
	}
	fmt.Fprintln(w, "\n## Summary")
	fmt.Fprintln(w, "\n| Before | After | Unchanged | Added | Removed | Modified |")
	fmt.Fprintln(w, "| ---: | ---: | ---: | ---: | ---: | ---: |")
	fmt.Fprintf(w, "| %d | %d | %d | %d | %d | %d |\n", result.Summary.Before, result.Summary.After, result.Summary.Unchanged, result.Summary.Added, result.Summary.Removed, result.Summary.Modified)
	renderMarkdownRoutes(w, "Added", result.Added)
	renderMarkdownRoutes(w, "Removed", result.Removed)
	fmt.Fprintf(w, "\n## Modified (%d)\n", len(result.Modified))
	for _, change := range result.Modified {
		fmt.Fprintf(w, "\n### `%s` — %s\n", markdownEscape(change.Before.Destination), markdownEscape(change.Before.Table))
		fmt.Fprintf(w, "\nChanged fields: %s\n", markdownEscape(strings.Join(change.ChangedFields, ", ")))
		fmt.Fprintln(w, "\n| | Protocol | Preference | Next hops |")
		fmt.Fprintln(w, "| --- | --- | --- | --- |")
		fmt.Fprintf(w, "| Before | %s | %s | %s |\n", markdownEscape(change.Before.Protocol), markdownEscape(change.Before.Preference), markdownEscape(formatNextHops(change.Before.NextHops)))
		fmt.Fprintf(w, "| After | %s | %s | %s |\n", markdownEscape(change.After.Protocol), markdownEscape(change.After.Preference), markdownEscape(formatNextHops(change.After.NextHops)))
	}
	fmt.Fprintln(w, "\n---")
	fmt.Fprintf(w, "Generated by routecompare %s. Input SHA-256: `%s` / `%s`.\n", result.Metadata.ToolVersion, result.Before.SHA256, result.After.SHA256)
	return nil
}

func renderMarkdownRoutes(w io.Writer, title string, routes []routecompare.Route) {
	fmt.Fprintf(w, "\n## %s (%d)\n", title, len(routes))
	if len(routes) == 0 {
		return
	}
	fmt.Fprintln(w, "\n| Destination | Table | Protocol | Preference | Next hops |")
	fmt.Fprintln(w, "| --- | --- | --- | ---: | --- |")
	for _, route := range routes {
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", markdownEscape(route.Destination), markdownEscape(route.Table), markdownEscape(route.Protocol), markdownEscape(route.Preference), markdownEscape(formatNextHops(route.NextHops)))
	}
}

func markdownEscape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

var htmlReportTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"hops": formatNextHops,
	"join": func(values []string) string { return strings.Join(values, ", ") },
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Route comparison report</title>
<style>
:root{color-scheme:light;--ink:#172033;--muted:#667085;--line:#dfe4ec;--panel:#fff;--bg:#f4f7fb;--blue:#175cd3;--green:#067647;--red:#b42318;--amber:#b54708}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}.wrap{max-width:1180px;margin:0 auto;padding:40px 24px 64px}header{display:flex;justify-content:space-between;gap:24px;align-items:flex-start;margin-bottom:28px}h1{font-size:30px;margin:0 0 6px}h2{font-size:20px;margin:32px 0 12px}.muted{color:var(--muted)}.status{padding:8px 12px;border-radius:999px;font-weight:700;background:#fff1f0;color:var(--red)}.status.pass{background:#ecfdf3;color:var(--green)}.meta,.cards,.inputs{display:grid;gap:12px}.meta{grid-template-columns:repeat(auto-fit,minmax(180px,1fr));margin:18px 0}.inputs{grid-template-columns:repeat(auto-fit,minmax(320px,1fr));margin:18px 0}.card,.input,.section{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:18px;box-shadow:0 1px 2px rgba(16,24,40,.04)}.cards{grid-template-columns:repeat(6,1fr)}.number{font-size:28px;font-weight:750}.label{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.04em}.added{color:var(--green)}.removed{color:var(--red)}.modified{color:var(--amber)}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;overflow-wrap:anywhere}table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:10px;border-bottom:1px solid var(--line);vertical-align:top}th{color:var(--muted);font-size:12px;text-transform:uppercase}.section{margin:12px 0;overflow-x:auto}.change{border-left:4px solid var(--amber)}.before{color:var(--red)}.after{color:var(--green)}@media(max-width:800px){.cards{grid-template-columns:repeat(2,1fr)}header{display:block}.status{display:inline-block;margin-top:12px}}@media print{body{background:#fff}.wrap{max-width:none;padding:12px}.card,.input,.section{box-shadow:none;break-inside:avoid}}
</style>
</head>
<body><main class="wrap">
<header><div><h1>Route comparison report</h1><div class="muted">Generated {{.Metadata.GeneratedAt}} by routecompare {{.Metadata.ToolVersion}}</div></div>{{if or .Summary.Added .Summary.Removed .Summary.Modified}}<div class="status">Changes detected</div>{{else}}<div class="status pass">No changes</div>{{end}}</header>
{{if or .Metadata.Device .Metadata.ChangeID}}<div class="meta">{{if .Metadata.Device}}<div><div class="label">Device</div><strong>{{.Metadata.Device}}</strong></div>{{end}}{{if .Metadata.ChangeID}}<div><div class="label">Change ID</div><strong>{{.Metadata.ChangeID}}</strong></div>{{end}}</div>{{end}}
<div class="inputs"><div class="input"><div class="label">Before snapshot</div><strong>{{.Before.Path}}</strong><br><code>{{.Before.SHA256}}</code></div><div class="input"><div class="label">After snapshot</div><strong>{{.After.Path}}</strong><br><code>{{.After.SHA256}}</code></div></div>
<div class="cards"><div class="card"><div class="number">{{.Summary.Before}}</div><div class="label">Before</div></div><div class="card"><div class="number">{{.Summary.After}}</div><div class="label">After</div></div><div class="card"><div class="number">{{.Summary.Unchanged}}</div><div class="label">Unchanged</div></div><div class="card"><div class="number added">{{.Summary.Added}}</div><div class="label">Added</div></div><div class="card"><div class="number removed">{{.Summary.Removed}}</div><div class="label">Removed</div></div><div class="card"><div class="number modified">{{.Summary.Modified}}</div><div class="label">Modified</div></div></div>
<h2>Added routes ({{len .Added}})</h2><div class="section"><table><thead><tr><th>Destination</th><th>Table</th><th>Protocol</th><th>Preference</th><th>Next hops</th></tr></thead><tbody>{{range .Added}}<tr><td><code>{{.Destination}}</code></td><td>{{.Table}}</td><td>{{.Protocol}}</td><td>{{.Preference}}</td><td>{{hops .NextHops}}</td></tr>{{else}}<tr><td colspan="5" class="muted">No displayed routes</td></tr>{{end}}</tbody></table></div>
<h2>Removed routes ({{len .Removed}})</h2><div class="section"><table><thead><tr><th>Destination</th><th>Table</th><th>Protocol</th><th>Preference</th><th>Next hops</th></tr></thead><tbody>{{range .Removed}}<tr><td><code>{{.Destination}}</code></td><td>{{.Table}}</td><td>{{.Protocol}}</td><td>{{.Preference}}</td><td>{{hops .NextHops}}</td></tr>{{else}}<tr><td colspan="5" class="muted">No displayed routes</td></tr>{{end}}</tbody></table></div>
<h2>Modified routes ({{len .Modified}})</h2>{{range .Modified}}<div class="section change"><strong><code>{{.Before.Destination}}</code></strong> · {{.Before.Table}}<div class="muted">Changed: {{join .ChangedFields}}</div><table><thead><tr><th></th><th>Protocol</th><th>Preference</th><th>Next hops</th></tr></thead><tbody><tr class="before"><td>Before</td><td>{{.Before.Protocol}}</td><td>{{.Before.Preference}}</td><td>{{hops .Before.NextHops}}</td></tr><tr class="after"><td>After</td><td>{{.After.Protocol}}</td><td>{{.After.Preference}}</td><td>{{hops .After.NextHops}}</td></tr></tbody></table></div>{{else}}<div class="section muted">No displayed routes</div>{{end}}
</main></body></html>`))

func renderHTML(w io.Writer, result report) error {
	if err := htmlReportTemplate.Execute(w, result); err != nil {
		return fmt.Errorf("write HTML report: %w", err)
	}
	return nil
}

func formatNextHops(nextHops []routecompare.NextHop) string {
	hops := make([]string, len(nextHops))
	for i, hop := range nextHops {
		hops[i] = strings.Trim(strings.Join([]string{hop.To, hop.Via, hop.LocalInterface}, " "), " ")
	}
	return strings.Join(hops, ", ")
}
