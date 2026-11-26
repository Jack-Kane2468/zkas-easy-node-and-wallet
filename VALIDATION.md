# Validation — v0.9.33-preview

Executed using Go 1.26.10 on Linux:

- go test ./internal/... passed.
- New regression checks cover legacy/default configurations, saved option persistence, launch arguments, retained loopback/no-custodial settings, invalid ranges, unsupported CLI flags, and a non-mutating disk inventory.
- GOOS=windows GOARCH=amd64 go vet ./... passed.
- Windows manager test executable compiled.
- Windows amd64 GUI executable rebuilt with embedded manifest and CGO disabled.
- Nested runnable ZIP and source ZIP checksums and archive integrity verified.
- Flag spelling/behavior reviewed against the official hash-verified v1.0.14 daemon --help and released upstream source.

Not executed here: native Windows UI/lifecycle tests, real-wallet checkpoint migration, live wallet/node operation, or measured disk/RAM savings. No user wallet files were altered. Run the native GitHub Windows checks and a backed-up local smoke test before publishing as stable.

Manual smoke checks: old config remains opt-out; supported flags start after Save and restart; unsupported daemon produces a clear error; resource controls cannot be changed while services run; disk review does not alter files; advanced panel fits compact mode; TG and desktop wallet flows still operate independently.
