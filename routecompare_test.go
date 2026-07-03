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

func TestParseRejectsInvalidXML(t *testing.T) {
	if _, err := routecompare.Parse(strings.NewReader(`<rpc-reply>`)); err == nil {
		t.Fatal("expected malformed XML error")
	}
}
