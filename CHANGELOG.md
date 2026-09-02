# Changelog

All notable changes to route-compare are documented here.

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
