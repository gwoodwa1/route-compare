# Changelog

All notable changes to route-compare are documented here.

## v1.2.0

### Added

- JSON comparison policies with table, protocol, prefix, and display filters.
- Policy limits for added, removed, and modified routes.
- Critical-prefix policy checks and policy results in every report format.
- Configurable ignored comparison fields through `-ignore` or a policy.
- Batch manifests with paths resolved relative to the manifest.
- JUnit XML output for individual and batch CI workflows.
- Route state, metrics, local preference, AS path, communities, tags, MPLS
  labels, and selected next-hop state in parsing and comparison.
- Extended fixture coverage and example policy and batch files.
- GitHub Actions for tests, race coverage, fuzzing, vulnerability scanning,
  static analysis, CodeQL, and tag-driven releases.
- GoReleaser cross-platform archives, checksums, CycloneDX SBOMs, and
  draft-before-publish binary verification.
- Grouped weekly Dependabot updates for Go modules and GitHub Actions.
- Hardened multi-stage container image with pinned builder and runtime digests,
  a non-root runtime user, container smoke tests, embedded binary scanning,
  Trivy image scanning, and Docker Dependabot updates.

### Changed

- Report tables include next-hop type and extended route attributes.
- Structured reports expose their effective pass/fail outcome.

## v1.1.0

### Added

- Added, removed, and modified route classification with changed-field details.
- Deterministic route ordering and comparison summary counts.
- JSON, Markdown, and self-contained HTML reports.
- Report metadata including UTC generation time, tool version, device, change
  identifier, input paths, and SHA-256 hashes.
- Filters for routing tables, protocols, covering IP prefixes, and displayed
  change types.
- `-output` support for writing reports directly to a file.
- `-fail-on` policies and exit status 2 for automation.
- Snapshot table discovery and missing-table validation APIs.
- Rich IPv4, IPv6, ECMP, protocol, and VRF fixtures.

### Changed

- XML without Junos route information or with malformed route records is now
  rejected instead of being treated as an empty snapshot.
- Terminal output now presents a summary and structured change categories.

### Compatibility

- `Difference.BeforeOnly` and `Difference.AfterOnly` remain available for v1.0
  package callers.

## v1.0.0

- Initial Junos XML parsing, route-table filtering, comparison library, and CLI.
