# route-compare

[![Test](https://github.com/gwoodwa1/route-compare/actions/workflows/test.yml/badge.svg)](https://github.com/gwoodwa1/route-compare/actions/workflows/test.yml)
[![CodeQL](https://github.com/gwoodwa1/route-compare/actions/workflows/codeql.yml/badge.svg)](https://github.com/gwoodwa1/route-compare/actions/workflows/codeql.yml)

`route-compare` is a Go library and command-line tool for comparing Junos route-table snapshots exported with `show route | display xml`. It classifies added, removed, and modified routes and produces terminal, JSON, Markdown, or self-contained HTML reports.

## Release

Current release: **v1.2.0**

See [CHANGELOG.md](CHANGELOG.md) for release details.

Requirements: Go 1.27.0 or later. The module uses only the Go standard library.

## Install the CLI

```sh
go install github.com/gwoodwa1/route-compare/cmd/routecompare@v1.2.0
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

Ignore volatile fields when they are not relevant to a validation:

```sh
routecompare \
  -pre testdata/fixtures/pre_2.xml \
  -post testdata/fixtures/post_2.xml \
  -ignore metric,metric2,communities
```

Apply a reusable policy containing filters, ignored fields, thresholds, and
critical prefixes:

```sh
routecompare \
  -pre testdata/fixtures/pre_2.xml \
  -post testdata/fixtures/post_2.xml \
  -policy testdata/policies/maintenance.json \
  -format html \
  -output policy-report.html
```

Run several comparisons from one manifest and emit CI-compatible JUnit XML:

```sh
routecompare \
  -batch testdata/batch.json \
  -format junit \
  -output routecompare-results.xml
```

Batch manifests support text, JSON, Markdown, and JUnit output. Snapshot and
policy paths inside a manifest are resolved relative to that manifest.

By default, detected changes are reported with a successful exit status. For
automation, `-fail-on` makes a matching comparison result exit with status 2:

```sh
routecompare -pre testdata/fixtures/pre_1.xml -post testdata/fixtures/post_1.xml -fail-on any
routecompare -pre testdata/fixtures/pre_1.xml -post testdata/fixtures/post_1.xml -fail-on removed
```

Valid policies are `none` (the default), `any`, `added`, `removed`, and
`modified`. Execution and input errors use exit status 1.

Run `routecompare -version` to print the installed version.

## Container image

Build the hardened multi-stage image:

```sh
docker build --build-arg VERSION=1.2.0 -t routecompare:1.2.0 .
```

Compare snapshots by mounting an input directory read-only:

```sh
docker run --rm \
  -v "$PWD/testdata:/workspace/testdata:ro" \
  routecompare:1.2.0 \
  -pre testdata/fixtures/pre_2.xml \
  -post testdata/fixtures/post_2.xml
```

Write an HTML report through the non-root container user:

```sh
mkdir -p reports
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -v "$PWD/testdata:/workspace/testdata:ro" \
  -v "$PWD/reports:/workspace/reports" \
  routecompare:1.2.0 \
  -pre testdata/fixtures/pre_2.xml \
  -post testdata/fixtures/post_2.xml \
  -format html \
  -output reports/route-report.html
```

The runtime image uses pinned Alpine packages, contains only the compiled
binary, CA certificates, and timezone data, and runs as the unprivileged
`routecompare` user.

## Use as a package

```sh
go get github.com/gwoodwa1/route-compare@v1.2.0
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
- `Comparator.IgnoreFields` can suppress selected comparison attributes.
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

## Automation and releases

GitHub Actions provides:

- Tests, vet, race detection, coverage, parser fuzzing, CLI smoke tests,
  `govulncheck`, `gosec`, and compiled-binary vulnerability scanning.
- Container build and smoke tests, embedded binary scanning, and Trivy runtime
  package scanning.
- CodeQL scanning on pushes, pull requests, and a weekly schedule.
- Tag-driven GoReleaser builds for Linux, macOS, and Windows on amd64 and
  arm64, with checksums and a CycloneDX SBOM.
- Draft-first releases whose binaries are scanned before publication.
- Weekly grouped Dependabot updates for Go modules and GitHub Actions.
- Weekly Docker base-image updates.

The release workflow requires the tag, library version, and CLI version to
match. After updating `Version` and `CHANGELOG.md`, create and push a semantic
version tag such as `v1.2.0`. The workflow tests and scans the project, creates
a draft GitHub release, verifies every binary, and publishes only after all
checks pass.

The package keeps XML decoding, route projection, comparison, and CLI presentation separate. Exported values are safe for callers to retain and modify without changing a parsed snapshot.

## Report metadata

Every structured report records its UTC generation time, routecompare version,
input paths, and SHA-256 hashes. `-device` and `-change-id` add optional
operational context. JSON reports also include the active table, protocol,
prefix, and change-type filters.

## Compared route attributes

The comparison model includes protocol, preference, next-hop type and paths,
active and hidden state, metrics, BGP local preference, AS path, communities,
route tag, selected next-hop state, and MPLS labels. Next-hop and community
ordering is normalized before comparison.

## Test fixtures

Example Junos snapshots are stored in `testdata/fixtures`:

- `pre_1.xml` and `post_1.xml` are a small next-hop-change example.
- `pre_2.xml` and `post_2.xml` contain IPv4, IPv6, multiple protocols,
  ECMP, private tables, and the `blue.inet.0` VRF. The post-change snapshot
  includes added, removed, and modified routes.

## License

GNU General Public License v3.0. See [LICENSE](LICENSE).
