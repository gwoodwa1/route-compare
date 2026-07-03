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
const Version = "1.0.0"

// NextHop identifies one forwarding path for a route.
type NextHop struct {
	To             string
	Via            string
	LocalInterface string
}

// Route is the comparison-friendly representation of a Junos route entry.
type Route struct {
	Destination string
	Table       string
	Protocol    string
	Preference  string
	NextHopType string
	NextHops    []NextHop
}

// Snapshot contains the route tables parsed from one rpc-reply.
type Snapshot struct {
	tables []routeTable
}

type rpcReply struct {
	RouteInformation struct {
		Tables []routeTable `xml:"route-table"`
	} `xml:"route-information"`
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

// Difference holds routes found on only one side of a comparison.
type Difference struct {
	BeforeOnly []Route
	AfterOnly  []Route
}

// Empty reports whether the snapshots contain equivalent routes.
func (d Difference) Empty() bool { return len(d.BeforeOnly) == 0 && len(d.AfterOnly) == 0 }

// Comparator compares route snapshots. It is stateless and safe for concurrent use.
type Comparator struct{}

// Compare finds routes unique to before and after. Next-hop order is ignored,
// while duplicate route entries are counted correctly.
func (Comparator) Compare(before, after []Route) Difference {
	return Difference{
		BeforeOnly: subtract(before, after),
		AfterOnly:  subtract(after, before),
	}
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
	hops := make([]string, len(r.NextHops))
	for i, hop := range r.NextHops {
		hops[i] = hop.To + "\x00" + hop.Via + "\x00" + hop.LocalInterface
	}
	sort.Strings(hops)
	return r.Table + "\x00" + r.Destination + "\x00" + r.Protocol + "\x00" + r.Preference + "\x00" + r.NextHopType + "\x00" + fmt.Sprint(hops)
}
