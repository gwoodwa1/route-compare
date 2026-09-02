# route-compare

`route-compare` is a Go library and command-line tool for comparing Junos route-table snapshots exported with `show route | display xml`. It classifies added, removed, and modified routes and produces terminal, JSON, Markdown, or self-contained HTML reports.

## Release

Current release: **v1.1.0**

See [CHANGELOG.md](CHANGELOG.md) for release details.

Requirements: Go 1.22 or later. The module uses only the Go standard library.

## Install the CLI

```sh
go install github.com/gwoodwa1/route-compare/cmd/routecompare@v1.1.0
```

Compare all routing tables:

```sh
routecompare -pre testdata/fixtures/pre_1.xml -post testdata/fixtures/post_1.xml
```

Compare selected routing tables:

```sh
routecompare -pre testdata/fixtures/pre_2.xml -post testdata/fixtures/post_2.xml -vrf inet.0,inet6.0
```

Produce a machine-readable report:

```sh
routecompare -pre testdata/fixtures/pre_1.xml -post testdata/fixtures/post_1.xml -format json
```

Create a shareable, self-contained HTML report with audit metadata:

```sh
routecompare \
  -pre testdata/fixtures/pre_2.xml \
  -post testdata/fixtures/post_2.xml \
  -format html \
  -output route-report.html \
  -device edge-01 \
  -change-id CHG-123
```

Create a focused Markdown report containing only BGP and static changes under
selected address ranges:

```sh
routecompare \
  -pre testdata/fixtures/pre_2.xml \
  -post testdata/fixtures/post_2.xml \
  -vrf inet.0,inet6.0 \
  -protocol BGP,Static \
  -prefix 0.0.0.0/0,::/0 \
  -change-type modified \
  -format markdown
```

`-prefix` includes routes contained by any supplied IPv4 or IPv6 prefix.
`-change-type` controls which detail sections are displayed; summary counts and
`-fail-on` continue to describe the complete filtered comparison.

By default, detected changes are reported with a successful exit status. For
automation, `-fail-on` makes a matching comparison result exit with status 2:

```sh
routecompare -pre testdata/fixtures/pre_1.xml -post testdata/fixtures/post_1.xml -fail-on any
routecompare -pre testdata/fixtures/pre_1.xml -post testdata/fixtures/post_1.xml -fail-on removed
```

Valid policies are `none` (the default), `any`, `added`, `removed`, and
`modified`. Execution and input errors use exit status 1.

Run `routecompare -version` to print the installed version.

## Use as a package

```sh
go get github.com/gwoodwa1/route-compare@v1.1.0
```

```go
package main

import (
	"fmt"
	"log"

	routecompare "github.com/gwoodwa1/route-compare"
)

func main() {
	before, err := routecompare.ParseFile("testdata/fixtures/pre_1.xml")
	if err != nil {
		log.Fatal(err)
	}
	after, err := routecompare.ParseFile("testdata/fixtures/post_1.xml")
	if err != nil {
		log.Fatal(err)
	}

	diff := (routecompare.Comparator{}).Compare(
		before.Routes("inet.0"),
		after.Routes("inet.0"),
	)
	fmt.Printf("added: %d\n", len(diff.Added))
	fmt.Printf("removed: %d\n", len(diff.Removed))
	fmt.Printf("modified: %d\n", len(diff.Modified))
}
```

`Parse(io.Reader)` is available when XML comes from memory, a network connection, or another stream. `Snapshot.Routes()` returns all tables; pass one or more table names to filter. Next-hop ordering is ignored during comparison, but duplicate route entries are preserved.

## Public API

- `Parse(io.Reader)` and `ParseFile(string)` create a `Snapshot`.
- Parsing rejects XML without route information, unnamed tables, and malformed
  route records instead of silently treating them as empty snapshots.
- `(*Snapshot).Routes(...string)` returns normalized routes.
- `(*Snapshot).TableNames()` lists the available routing tables, and
  `(*Snapshot).MissingTables(...string)` validates requested tables.
- `(Comparator).Compare(before, after)` returns a `Difference`.
- `Difference.Added`, `Removed`, and `Modified` classify changes. A modified
  route has the same table and destination on both sides, with its changed
  fields listed in `RouteChange.ChangedFields`.
- `Difference.BeforeOnly` and `AfterOnly` remain available for compatibility
  with v1.0 callers.
- `(Difference).Empty()` reports whether two inputs are equivalent.

## Development

```sh
go test ./...
go vet ./...
```

The package keeps XML decoding, route projection, comparison, and CLI presentation separate. Exported values are safe for callers to retain and modify without changing a parsed snapshot.

## Report metadata

Every structured report records its UTC generation time, routecompare version,
input paths, and SHA-256 hashes. `-device` and `-change-id` add optional
operational context. JSON reports also include the active table, protocol,
prefix, and change-type filters.

## Test fixtures

Example Junos snapshots are stored in `testdata/fixtures`:

- `pre_1.xml` and `post_1.xml` are a small next-hop-change example.
- `pre_2.xml` and `post_2.xml` contain IPv4, IPv6, multiple protocols,
  ECMP, private tables, and the `blue.inet.0` VRF. The post-change snapshot
  includes added, removed, and modified routes.

## License

GNU General Public License v3.0. See [LICENSE](LICENSE).
