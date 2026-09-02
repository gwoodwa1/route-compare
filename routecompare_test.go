package routecompare_test

import (
	"strings"
	"testing"

	routecompare "github.com/gwoodwa1/route-compare"
)

func TestParseAndFilterRoutes(t *testing.T) {
	xml := `<rpc-reply><route-information><route-table><table-name>inet.0</table-name><rt><rt-destination>192.0.2.0/24</rt-destination><rt-entry><protocol-name>Static</protocol-name><preference>5</preference><nh><to>192.0.2.1</to><via>ge-0/0/0.0</via></nh></rt-entry></rt></route-table><route-table><table-name>inet6.0</table-name><rt><rt-destination>::/0</rt-destination><rt-entry><protocol-name>Static</protocol-name></rt-entry></rt></route-table></route-information></rpc-reply>`
	snapshot, err := routecompare.Parse(strings.NewReader(xml))
	if err != nil {
		t.Fatal(err)
	}
	routes := snapshot.Routes("inet.0")
	if len(routes) != 1 || routes[0].Destination != "192.0.2.0/24" || routes[0].NextHops[0].Via != "ge-0/0/0.0" {
		t.Fatalf("unexpected routes: %#v", routes)
	}
}

func TestCompareIgnoresNextHopOrderAndCountsDuplicates(t *testing.T) {
	a := routecompare.Route{Destination: "192.0.2.0/24", Table: "inet.0", NextHops: []routecompare.NextHop{{To: "192.0.2.1"}, {To: "192.0.2.2"}}}
	b := a
	b.NextHops = []routecompare.NextHop{{To: "192.0.2.2"}, {To: "192.0.2.1"}}
	diff := (routecompare.Comparator{}).Compare([]routecompare.Route{a, a}, []routecompare.Route{b})
	if len(diff.BeforeOnly) != 1 || len(diff.AfterOnly) != 0 {
		t.Fatalf("unexpected difference: %#v", diff)
	}
}

func TestCompareClassifiesAddedRemovedAndModified(t *testing.T) {
	unchanged := routecompare.Route{Destination: "192.0.2.0/24", Table: "inet.0", Protocol: "Direct"}
	modifiedBefore := routecompare.Route{Destination: "198.51.100.0/24", Table: "inet.0", Protocol: "Static", Preference: "5", NextHops: []routecompare.NextHop{{To: "192.0.2.1"}}}
	modifiedAfter := modifiedBefore
	modifiedAfter.NextHops = []routecompare.NextHop{{To: "192.0.2.2"}}
	removed := routecompare.Route{Destination: "203.0.113.0/24", Table: "inet.0", Protocol: "BGP"}
	added := routecompare.Route{Destination: "2001:db8::/32", Table: "inet6.0", Protocol: "Static"}

	diff := (routecompare.Comparator{}).Compare(
		[]routecompare.Route{removed, modifiedBefore, unchanged},
		[]routecompare.Route{added, unchanged, modifiedAfter},
	)
	if diff.BeforeCount != 3 || diff.AfterCount != 3 || diff.UnchangedCount != 1 {
		t.Fatalf("unexpected counts: %#v", diff)
	}
	if len(diff.Added) != 1 || diff.Added[0].Destination != added.Destination {
		t.Fatalf("unexpected added routes: %#v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Destination != removed.Destination {
		t.Fatalf("unexpected removed routes: %#v", diff.Removed)
	}
	if len(diff.Modified) != 1 {
		t.Fatalf("unexpected modified routes: %#v", diff.Modified)
	}
	change := diff.Modified[0]
	if change.Before.NextHops[0].To != "192.0.2.1" || change.After.NextHops[0].To != "192.0.2.2" {
		t.Fatalf("unexpected modification: %#v", change)
	}
	if len(change.ChangedFields) != 1 || change.ChangedFields[0] != "next_hops" {
		t.Fatalf("unexpected changed fields: %#v", change.ChangedFields)
	}
}

func TestCompareReturnsDeterministicOrder(t *testing.T) {
	routes := []routecompare.Route{
		{Destination: "203.0.113.0/24", Table: "inet.0", Protocol: "Static"},
		{Destination: "192.0.2.0/24", Table: "inet.0", Protocol: "Static"},
		{Destination: "2001:db8::/32", Table: "inet6.0", Protocol: "Static"},
	}
	diff := (routecompare.Comparator{}).Compare(nil, routes)
	got := []string{diff.Added[0].Destination, diff.Added[1].Destination, diff.Added[2].Destination}
	want := []string{"192.0.2.0/24", "203.0.113.0/24", "2001:db8::/32"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("added order = %#v, want %#v", got, want)
		}
	}
}

func TestParseExtendedRouteAttributes(t *testing.T) {
	input := `<rpc-reply><route-information><route-table><table-name>inet.0</table-name><rt><rt-destination>192.0.2.0/24</rt-destination><rt-entry><current-active/><protocol-name>BGP</protocol-name><preference>170</preference><metric>20</metric><metric2>30</metric2><local-preference>200</local-preference><as-path>64500 64501 I</as-path><communities><community>64500:100</community><community>64500:200</community></communities><tag>42</tag><nh><selected-next-hop/><to>198.51.100.1</to><via>ge-0/0/0.0</via><mpls-label>Push 100</mpls-label></nh></rt-entry></rt></route-table></route-information></rpc-reply>`
	snapshot, err := routecompare.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	route := snapshot.Routes()[0]
	if !route.Active || route.Metric != "20" || route.Metric2 != "30" || route.LocalPreference != "200" || route.ASPath != "64500 64501 I" || route.Tag != "42" {
		t.Fatalf("unexpected extended attributes: %#v", route)
	}
	if len(route.Communities) != 2 || !route.NextHops[0].Selected || route.NextHops[0].Label != "Push 100" {
		t.Fatalf("unexpected communities or next hop: %#v", route)
	}
}

func TestComparatorIgnoreFields(t *testing.T) {
	before := routecompare.Route{Destination: "192.0.2.0/24", Table: "inet.0", Protocol: "BGP", Preference: "170", Metric: "10"}
	after := before
	after.Preference = "200"
	after.Metric = "20"

	diff := (routecompare.Comparator{IgnoreFields: []string{"preference"}}).Compare([]routecompare.Route{before}, []routecompare.Route{after})
	if len(diff.Modified) != 1 || len(diff.Modified[0].ChangedFields) != 1 || diff.Modified[0].ChangedFields[0] != "metric" {
		t.Fatalf("unexpected ignored-field comparison: %#v", diff)
	}
	diff = (routecompare.Comparator{IgnoreFields: []string{"preference", "metric"}}).Compare([]routecompare.Route{before}, []routecompare.Route{after})
	if !diff.Empty() || diff.UnchangedCount != 1 {
		t.Fatalf("ignored fields still produced a difference: %#v", diff)
	}
}

func TestParseRejectsInvalidXML(t *testing.T) {
	if _, err := routecompare.Parse(strings.NewReader(`<rpc-reply>`)); err == nil {
		t.Fatal("expected malformed XML error")
	}
}

func TestParseRejectsXMLWithoutRouteData(t *testing.T) {
	tests := []string{
		`<rpc-reply><system-information/></rpc-reply>`,
		`<rpc-reply><route-information/></rpc-reply>`,
		`<rpc-reply><route-information><route-table><rt/></route-table></route-information></rpc-reply>`,
	}
	for _, input := range tests {
		if _, err := routecompare.Parse(strings.NewReader(input)); err == nil {
			t.Fatalf("expected validation error for %s", input)
		}
	}
}

func TestSnapshotTableNamesAndMissingTables(t *testing.T) {
	input := `<rpc-reply><route-information><route-table><table-name>inet.0</table-name></route-table><route-table><table-name>blue.inet.0</table-name></route-table></route-information></rpc-reply>`
	snapshot, err := routecompare.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	names := snapshot.TableNames()
	if len(names) != 2 || names[0] != "inet.0" || names[1] != "blue.inet.0" {
		t.Fatalf("unexpected table names: %#v", names)
	}
	names[0] = "changed"
	if snapshot.TableNames()[0] != "inet.0" {
		t.Fatal("TableNames returned internal storage")
	}
	missing := snapshot.MissingTables("inet.0", "red.inet.0", "red.inet.0")
	if len(missing) != 1 || missing[0] != "red.inet.0" {
		t.Fatalf("unexpected missing tables: %#v", missing)
	}
}

func TestFixtureComparisons(t *testing.T) {
	tests := []struct {
		name                                string
		before, after                       string
		unchanged, added, removed, modified int
	}{
		{
			name:      "small next-hop change",
			before:    "testdata/fixtures/pre_1.xml",
			after:     "testdata/fixtures/post_1.xml",
			unchanged: 3,
			modified:  1,
		},
		{
			name:      "mixed route changes",
			before:    "testdata/fixtures/pre_2.xml",
			after:     "testdata/fixtures/post_2.xml",
			unchanged: 21,
			added:     2,
			removed:   2,
			modified:  5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, err := routecompare.ParseFile(tt.before)
			if err != nil {
				t.Fatal(err)
			}
			after, err := routecompare.ParseFile(tt.after)
			if err != nil {
				t.Fatal(err)
			}
			diff := (routecompare.Comparator{}).Compare(before.Routes(), after.Routes())
			if diff.UnchangedCount != tt.unchanged || len(diff.Added) != tt.added || len(diff.Removed) != tt.removed || len(diff.Modified) != tt.modified {
				t.Fatalf("unexpected fixture comparison: unchanged=%d added=%d removed=%d modified=%d", diff.UnchangedCount, len(diff.Added), len(diff.Removed), len(diff.Modified))
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(`<rpc-reply><route-information><route-table><table-name>inet.0</table-name><rt><rt-destination>192.0.2.0/24</rt-destination><rt-entry><protocol-name>Static</protocol-name><preference>5</preference><nh><to>192.0.2.1</to></nh></rt-entry></rt></route-table></route-information></rpc-reply>`))
	f.Add([]byte(`<rpc-reply>`))
	f.Fuzz(func(t *testing.T, input []byte) {
		snapshot, err := routecompare.Parse(strings.NewReader(string(input)))
		if err != nil {
			return
		}
		_ = snapshot.TableNames()
		_ = snapshot.Routes()
	})
}
