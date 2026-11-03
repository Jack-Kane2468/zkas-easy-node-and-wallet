# Validation — v0.9.31-preview

Verified on Linux with Go 1.26.10:

- go test ./internal/... passed, including notification category toggles, save-before-apply behavior, save failure, owner authorization, wallet permission checks, invalid schedule/threshold rejection, explicit-state replay and callback sender binding.
- GOOS=windows GOARCH=amd64 go vet ./... passed.
- Windows amd64 GUI executable rebuilt with its embedded manifest, CGO disabled and stripped GUI linker flags.
- Windows-target govulncheck v1.8.0: no vulnerabilities found.
- Complete source and runnable ZIP integrity and SHA-256 manifests verified.
- Compared source with v0.9.30: mining tab and bridge-update code are unchanged.

Not executed here: native Windows UI/lifecycle tests or live Telegram delivery. The Windows notification persistence/UI integration and monitoring loop were compiled and vetted, but require a live smoke test. No wallet transaction or mining execution was performed.
