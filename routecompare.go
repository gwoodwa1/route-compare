// Package routecompare parses Junos XML route-table snapshots and compares them.
package routecompare

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
)

// Version is the semantic version of this release.
const Version = "1.1.0"

// NextHop identifies one forwarding path for a route.
type NextHop struct {
	To             string `json:"to,omitempty"`
	Via            string `json:"via,omitempty"`
	LocalInterface string `json:"local_interface,omitempty"`
}

// Route is the comparison-friendly representation of a Junos route entry.
type Route struct {
	Destination string    `json:"destination"`
	Table       string    `json:"table"`
	Protocol    string    `json:"protocol,omitempty"`
	Preference  string    `json:"preference,omitempty"`
	NextHopType string    `json:"next_hop_type,omitempty"`
	NextHops    []NextHop `json:"next_hops,omitempty"`
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
	Protocol    string       `xml:"protocol-name"`
	Preference  string       `xml:"preference"`
	NextHopType string       `xml:"nh-type"`
	NextHops    []xmlNextHop `xml:"nh"`
}

type xmlNextHop struct {
	To             string `xml:"to"`
	Via            string `xml:"via"`
	LocalInterface string `xml:"nh-local-interface"`
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
					Destination: candidate.Destination,
					Table:       table.Name,
					Protocol:    entry.Protocol,
					Preference:  entry.Preference,
					NextHopType: entry.NextHopType,
					NextHops:    make([]NextHop, len(entry.NextHops)),
				}
				for i, hop := range entry.NextHops {
					route.NextHops[i] = NextHop{To: hop.To, Via: hop.Via, LocalInterface: hop.LocalInterface}
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

// Comparator compares route snapshots. It is stateless and safe for concurrent use.
type Comparator struct{}

// Compare finds routes unique to before and after. Next-hop order is ignored,
// while duplicate route entries are counted correctly.
func (Comparator) Compare(before, after []Route) Difference {
	beforeOnly := subtract(before, after)
	afterOnly := subtract(after, before)
	diff := Difference{
		BeforeOnly:     append([]Route(nil), beforeOnly...),
		AfterOnly:      append([]Route(nil), afterOnly...),
		BeforeCount:    len(before),
		AfterCount:     len(after),
		UnchangedCount: len(before) - len(beforeOnly),
	}
	diff.classify(beforeOnly, afterOnly)
	sortRoutes(diff.BeforeOnly)
	sortRoutes(diff.AfterOnly)
	sortRoutes(diff.Added)
	sortRoutes(diff.Removed)
	sort.SliceStable(diff.Modified, func(i, j int) bool {
		return routeLess(diff.Modified[i].Before, diff.Modified[j].Before)
	})
	return diff
}

func (d *Difference) classify(beforeOnly, afterOnly []Route) {
	used := make([]bool, len(afterOnly))
	for _, before := range beforeOnly {
		best := -1
		bestScore := int(^uint(0) >> 1)
		for i, after := range afterOnly {
			if used[i] || before.Table != after.Table || before.Destination != after.Destination {
				continue
			}
			fields := changedFields(before, after)
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
			ChangedFields: changedFields(before, after),
		})
	}
	for i, after := range afterOnly {
		if !used[i] {
			d.Added = append(d.Added, after)
		}
	}
}

func changedFields(before, after Route) []string {
	var fields []string
	if before.Protocol != after.Protocol {
		fields = append(fields, "protocol")
	}
	if before.Preference != after.Preference {
		fields = append(fields, "preference")
	}
	if before.NextHopType != after.NextHopType {
		fields = append(fields, "next_hop_type")
	}
	if nextHopKey(before.NextHops) != nextHopKey(after.NextHops) {
		fields = append(fields, "next_hops")
	}
	return fields
}

func subtract(left, right []Route) []Route {
	counts := make(map[string]int, len(right))
	for _, route := range right {
		counts[route.key()]++
	}
	var unique []Route
	for _, route := range left {
		key := route.key()
		if counts[key] > 0 {
			counts[key]--
			continue
		}
		unique = append(unique, route)
	}
	return unique
}

func (r Route) key() string {
	return r.Table + "\x00" + r.Destination + "\x00" + r.Protocol + "\x00" + r.Preference + "\x00" + r.NextHopType + "\x00" + nextHopKey(r.NextHops)
}

func nextHopKey(nextHops []NextHop) string {
	hops := make([]string, len(nextHops))
	for i, hop := range nextHops {
		hops[i] = hop.To + "\x00" + hop.Via + "\x00" + hop.LocalInterface
	}
	sort.Strings(hops)
	return fmt.Sprint(hops)
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
