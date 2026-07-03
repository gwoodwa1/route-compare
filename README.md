# route-compare

`route-compare` is a Go library and command-line tool for comparing Junos route-table snapshots exported with `show route | display xml`. It reports routes present only before or only after a change and supports filtering by routing table.

## Release

Current release: **v1.0.0**

Requirements: Go 1.22 or later. The module uses only the Go standard library.

## Install the CLI

```sh
go install github.com/gwoodwa1/route-compare/cmd/routecompare@v1.0.0
```

Compare all routing tables:

```sh
routecompare -pre pre.xml -post post.xml
```

Compare selected routing tables:

```sh
routecompare -pre pre.xml -post post.xml -vrf inet.0,inet6.0
```

Run `routecompare -version` to print the installed version.

## Use as a package

```sh
go get github.com/gwoodwa1/route-compare@v1.0.0
```

```go
package main

import (
	"fmt"
	"log"

	routecompare "github.com/gwoodwa1/route-compare"
)

func main() {
	before, err := routecompare.ParseFile("pre.xml")
	if err != nil {
		log.Fatal(err)
	}
	after, err := routecompare.ParseFile("post.xml")
	if err != nil {
		log.Fatal(err)
	}

	diff := (routecompare.Comparator{}).Compare(
		before.Routes("inet.0"),
		after.Routes("inet.0"),
	)
	fmt.Printf("removed or changed: %d\n", len(diff.BeforeOnly))
	fmt.Printf("added or changed: %d\n", len(diff.AfterOnly))
}
```

`Parse(io.Reader)` is available when XML comes from memory, a network connection, or another stream. `Snapshot.Routes()` returns all tables; pass one or more table names to filter. Next-hop ordering is ignored during comparison, but duplicate route entries are preserved.

## Public API

- `Parse(io.Reader)` and `ParseFile(string)` create a `Snapshot`.
- `(*Snapshot).Routes(...string)` returns normalized routes.
- `(Comparator).Compare(before, after)` returns a `Difference`.
- `(Difference).Empty()` reports whether two inputs are equivalent.

## Development

```sh
go test ./...
go vet ./...
```

The package keeps XML decoding, route projection, comparison, and CLI presentation separate. Exported values are safe for callers to retain and modify without changing a parsed snapshot.

## License

GNU General Public License v3.0. See [LICENSE](LICENSE).
