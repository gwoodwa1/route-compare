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

func TestParseRejectsInvalidXML(t *testing.T) {
	if _, err := routecompare.Parse(strings.NewReader(`<rpc-reply>`)); err == nil {
		t.Fatal("expected malformed XML error")
	}
}
