// Package routecompare parses Junos XML route-table snapshots and compares them.
package routecompare

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Version is the semantic version of this release.
const Version = "1.2.0"

// NextHop identifies one forwarding path for a route.
type NextHop struct {
	To             string `json:"to,omitempty"`
	Via            string `json:"via,omitempty"`
	LocalInterface string `json:"local_interface,omitempty"`
	Label          string `json:"label,omitempty"`
	Selected       bool   `json:"selected,omitempty"`
}

// Route is the comparison-friendly representation of a Junos route entry.
type Route struct {
	Destination     string    `json:"destination"`
	Table           string    `json:"table"`
	Protocol        string    `json:"protocol,omitempty"`
	Preference      string    `json:"preference,omitempty"`
	NextHopType     string    `json:"next_hop_type,omitempty"`
	NextHops        []NextHop `json:"next_hops,omitempty"`
	Active          bool      `json:"active,omitempty"`
	Hidden          bool      `json:"hidden,omitempty"`
	Metric          string    `json:"metric,omitempty"`
	Metric2         string    `json:"metric2,omitempty"`
	LocalPreference string    `json:"local_preference,omitempty"`
	ASPath          string    `json:"as_path,omitempty"`
	Communities     []string  `json:"communities,omitempty"`
	Tag             string    `json:"tag,omitempty"`
}

// Snapshot contains the route tables parsed from one rpc-reply.
type Snapshot struct {
	tables []routeTable
}

type rpcReply struct {
	RouteInformation *routeInformation `xml:"route-information"`
}

type routeInformation struct {
	Tables []routeTable `xml:"route-table"`
}

type routeTable struct {
	Name   string     `xml:"table-name"`
	Routes []xmlRoute `xml:"rt"`
}

type xmlRoute struct {
	Destination string     `xml:"rt-destination"`
	Entries     []xmlEntry `xml:"rt-entry"`
}

type xmlEntry struct {
	Protocol        string       `xml:"protocol-name"`
	Preference      string       `xml:"preference"`
	NextHopType     string       `xml:"nh-type"`
	NextHops        []xmlNextHop `xml:"nh"`
	CurrentActive   *struct{}    `xml:"current-active"`
	Hidden          *struct{}    `xml:"hidden"`
	Metric          string       `xml:"metric"`
	Metric2         string       `xml:"metric2"`
	LocalPreference string       `xml:"local-preference"`
	ASPath          string       `xml:"as-path"`
	Communities     []string     `xml:"communities>community"`
	Tag             string       `xml:"tag"`
}

type xmlNextHop struct {
	To             string    `xml:"to"`
	Via            string    `xml:"via"`
	LocalInterface string    `xml:"nh-local-interface"`
	Label          string    `xml:"mpls-label"`
	Selected       *struct{} `xml:"selected-next-hop"`
}

// Parse reads a Junos XML rpc-reply. The reader is not closed by Parse.
func Parse(r io.Reader) (*Snapshot, error) {
	if r == nil {
		return nil, fmt.Errorf("parse route snapshot: nil reader")
	}

	var reply rpcReply
	if err := xml.NewDecoder(r).Decode(&reply); err != nil {
		return nil, fmt.Errorf("parse route snapshot: %w", err)
	}
	if reply.RouteInformation == nil {
		return nil, fmt.Errorf("parse route snapshot: route-information element not found")
	}
	if len(reply.RouteInformation.Tables) == 0 {
		return nil, fmt.Errorf("parse route snapshot: no route tables found")
	}
	for tableIndex, table := range reply.RouteInformation.Tables {
		if table.Name == "" {
			return nil, fmt.Errorf("parse route snapshot: route table %d has no table-name", tableIndex+1)
		}
		for routeIndex, route := range table.Routes {
			if route.Destination == "" {
				return nil, fmt.Errorf("parse route snapshot: route %d in table %q has no destination", routeIndex+1, table.Name)
			}
			if len(route.Entries) == 0 {
				return nil, fmt.Errorf("parse route snapshot: route %q in table %q has no entries", route.Destination, table.Name)
			}
		}
	}
	return &Snapshot{tables: reply.RouteInformation.Tables}, nil
}

// ParseFile opens and parses a Junos XML rpc-reply.
func ParseFile(name string) (*Snapshot, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open route snapshot %q: %w", name, err)
	}
	defer f.Close()
	return Parse(f)
}

// TableNames returns the routing table names in source order. The returned
// slice is safe for the caller to modify.
func (s *Snapshot) TableNames() []string {
	if s == nil {
		return nil
	}
	names := make([]string, len(s.tables))
	for i, table := range s.tables {
		names[i] = table.Name
	}
	return names
}

// MissingTables returns requested routing table names not present in the
// snapshot. Empty names and ALL are ignored.
func (s *Snapshot) MissingTables(tableNames ...string) []string {
	present := make(map[string]struct{})
	if s != nil {
		for _, table := range s.tables {
			present[table.Name] = struct{}{}
		}
	}
	var missing []string
	seen := make(map[string]struct{})
	for _, name := range tableNames {
		if name == "" || name == "ALL" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		if _, ok := present[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// Routes returns a copy of the snapshot's routes. With no table names, routes
// from every routing table are returned.
func (s *Snapshot) Routes(tableNames ...string) []Route {
	if s == nil {
		return nil
	}
	wanted := make(map[string]struct{}, len(tableNames))
	for _, name := range tableNames {
		if name != "" && name != "ALL" {
			wanted[name] = struct{}{}
		}
	}

	var routes []Route
	for _, table := range s.tables {
		if len(wanted) != 0 {
			if _, ok := wanted[table.Name]; !ok {
				continue
			}
		}
		for _, candidate := range table.Routes {
			for _, entry := range candidate.Entries {
				route := Route{
					Destination:     candidate.Destination,
					Table:           table.Name,
					Protocol:        entry.Protocol,
					Preference:      entry.Preference,
					NextHopType:     entry.NextHopType,
					NextHops:        make([]NextHop, len(entry.NextHops)),
					Active:          entry.CurrentActive != nil,
					Hidden:          entry.Hidden != nil,
					Metric:          entry.Metric,
					Metric2:         entry.Metric2,
					LocalPreference: entry.LocalPreference,
					ASPath:          entry.ASPath,
					Communities:     append([]string(nil), entry.Communities...),
					Tag:             entry.Tag,
				}
				for i, hop := range entry.NextHops {
					route.NextHops[i] = NextHop{To: hop.To, Via: hop.Via, LocalInterface: hop.LocalInterface, Label: hop.Label, Selected: hop.Selected != nil}
				}
				routes = append(routes, route)
			}
		}
	}
	return routes
}

// Difference holds both the compatibility-oriented raw difference and the
// operator-friendly classification of a comparison.
type Difference struct {
	BeforeOnly []Route
	AfterOnly  []Route

	// Added and Removed contain routes that exist on only one side and do not
	// have a corresponding route with the same table and destination.
	Added   []Route
	Removed []Route

	// Modified pairs routes with the same table and destination whose
	// comparison-relevant attributes changed.
	Modified []RouteChange

	// Counts include exact matches, including duplicate entries.
	BeforeCount    int
	AfterCount     int
	UnchangedCount int
}

// RouteChange describes a route that exists in both snapshots but whose
// comparison-relevant attributes changed.
type RouteChange struct {
	Before        Route    `json:"before"`
	After         Route    `json:"after"`
	ChangedFields []string `json:"changed_fields"`
}

// Empty reports whether the snapshots contain equivalent routes.
func (d Difference) Empty() bool { return len(d.BeforeOnly) == 0 && len(d.AfterOnly) == 0 }

// Comparator compares route snapshots. IgnoreFields may contain protocol,
// preference, next_hop_type, next_hops, active, hidden, metric, metric2,
// local_preference, as_path, communities, or tag. It is safe for concurrent
// use when its configuration is not mutated.
type Comparator struct {
	IgnoreFields []string
}

// Compare finds routes unique to before and after. Next-hop order is ignored,
// while duplicate route entries are counted correctly.
func (c Comparator) Compare(before, after []Route) Difference {
	beforeOnly := subtract(before, after, c.routeKey)
	afterOnly := subtract(after, before, c.routeKey)
	diff := Difference{
		BeforeOnly:     append([]Route(nil), beforeOnly...),
		AfterOnly:      append([]Route(nil), afterOnly...),
		BeforeCount:    len(before),
		AfterCount:     len(after),
		UnchangedCount: len(before) - len(beforeOnly),
	}
	c.classify(&diff, beforeOnly, afterOnly)
	sortRoutes(diff.BeforeOnly)
	sortRoutes(diff.AfterOnly)
	sortRoutes(diff.Added)
	sortRoutes(diff.Removed)
	sort.SliceStable(diff.Modified, func(i, j int) bool {
		return routeLess(diff.Modified[i].Before, diff.Modified[j].Before)
	})
	return diff
}

func (c Comparator) classify(d *Difference, beforeOnly, afterOnly []Route) {
	used := make([]bool, len(afterOnly))
	for _, before := range beforeOnly {
		best := -1
		bestScore := int(^uint(0) >> 1)
		for i, after := range afterOnly {
			if used[i] || before.Table != after.Table || before.Destination != after.Destination {
				continue
			}
			fields := c.changedFields(before, after)
			if len(fields) < bestScore {
				best, bestScore = i, len(fields)
			}
		}
		if best == -1 {
			d.Removed = append(d.Removed, before)
			continue
		}
		used[best] = true
		after := afterOnly[best]
		d.Modified = append(d.Modified, RouteChange{
			Before:        before,
			After:         after,
			ChangedFields: c.changedFields(before, after),
		})
	}
	for i, after := range afterOnly {
		if !used[i] {
			d.Added = append(d.Added, after)
		}
	}
}

func (c Comparator) changedFields(before, after Route) []string {
	var fields []string
	if !c.ignores("protocol") && before.Protocol != after.Protocol {
		fields = append(fields, "protocol")
	}
	if !c.ignores("preference") && before.Preference != after.Preference {
		fields = append(fields, "preference")
	}
	if !c.ignores("next_hop_type") && before.NextHopType != after.NextHopType {
		fields = append(fields, "next_hop_type")
	}
	if !c.ignores("next_hops") && nextHopKey(before.NextHops) != nextHopKey(after.NextHops) {
		fields = append(fields, "next_hops")
	}
	if !c.ignores("active") && before.Active != after.Active {
		fields = append(fields, "active")
	}
	if !c.ignores("hidden") && before.Hidden != after.Hidden {
		fields = append(fields, "hidden")
	}
	if !c.ignores("metric") && before.Metric != after.Metric {
		fields = append(fields, "metric")
	}
	if !c.ignores("metric2") && before.Metric2 != after.Metric2 {
		fields = append(fields, "metric2")
	}
	if !c.ignores("local_preference") && before.LocalPreference != after.LocalPreference {
		fields = append(fields, "local_preference")
	}
	if !c.ignores("as_path") && before.ASPath != after.ASPath {
		fields = append(fields, "as_path")
	}
	if !c.ignores("communities") && stringSliceKey(before.Communities) != stringSliceKey(after.Communities) {
		fields = append(fields, "communities")
	}
	if !c.ignores("tag") && before.Tag != after.Tag {
		fields = append(fields, "tag")
	}
	return fields
}

func subtract(left, right []Route, key func(Route) string) []Route {
	counts := make(map[string]int, len(right))
	for _, route := range right {
		counts[key(route)]++
	}
	var unique []Route
	for _, route := range left {
		routeKey := key(route)
		if counts[routeKey] > 0 {
			counts[routeKey]--
			continue
		}
		unique = append(unique, route)
	}
	return unique
}

func (r Route) key() string {
	return (Comparator{}).routeKey(r)
}

func (c Comparator) routeKey(r Route) string {
	values := []string{r.Table, r.Destination}
	fields := []struct {
		name, value string
	}{
		{"protocol", r.Protocol},
		{"preference", r.Preference},
		{"next_hop_type", r.NextHopType},
		{"next_hops", nextHopKey(r.NextHops)},
		{"active", strconv.FormatBool(r.Active)},
		{"hidden", strconv.FormatBool(r.Hidden)},
		{"metric", r.Metric},
		{"metric2", r.Metric2},
		{"local_preference", r.LocalPreference},
		{"as_path", r.ASPath},
		{"communities", stringSliceKey(r.Communities)},
		{"tag", r.Tag},
	}
	for _, field := range fields {
		if !c.ignores(field.name) {
			values = append(values, field.value)
		}
	}
	return strings.Join(values, "\x00")
}

func (c Comparator) ignores(field string) bool {
	for _, ignored := range c.IgnoreFields {
		if strings.EqualFold(strings.TrimSpace(ignored), field) {
			return true
		}
	}
	return false
}

func nextHopKey(nextHops []NextHop) string {
	hops := make([]string, len(nextHops))
	for i, hop := range nextHops {
		hops[i] = hop.To + "\x00" + hop.Via + "\x00" + hop.LocalInterface + "\x00" + hop.Label + "\x00" + strconv.FormatBool(hop.Selected)
	}
	sort.Strings(hops)
	return fmt.Sprint(hops)
}

func stringSliceKey(values []string) string {
	copyOfValues := append([]string(nil), values...)
	sort.Strings(copyOfValues)
	return strings.Join(copyOfValues, "\x00")
}

func sortRoutes(routes []Route) {
	sort.SliceStable(routes, func(i, j int) bool { return routeLess(routes[i], routes[j]) })
}

func routeLess(left, right Route) bool {
	if left.Table != right.Table {
		return left.Table < right.Table
	}
	if left.Destination != right.Destination {
		return left.Destination < right.Destination
	}
	if left.Protocol != right.Protocol {
		return left.Protocol < right.Protocol
	}
	return left.key() < right.key()
}
