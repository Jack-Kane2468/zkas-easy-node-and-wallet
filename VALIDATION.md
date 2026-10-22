# v0.9.18-preview validation

Portable internal Go tests and Windows static checks passed. Windows x64 GUI executable cross-built. Regression tests cover unique bot-wallet matching, ambiguous checkpoint rejection, stale checkpoint rejection and recent-loop classification with distinct timestamps. Existing tests cover rescan request payloads and partial refusal.

Runtime hashes, archive integrity, executable version and SHA256 manifests are checked during packaging. Native Windows inline-button interaction, physical scrolling and funded live rescans have not been tested in this build environment.
