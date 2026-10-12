# v0.9.15-preview validation

- Portable Go tests passed across all internal packages.
- Rescan tests cover daemon registrations outside the vault, unchanged registration/checkpoint files, exact official API requests with the genesis check retained, invalid registration rejection and partial success followed by refusal.
- Windows go vet passed.
- Windows x64 GUI executable cross-built; PE architecture, GUI subsystem and version checked during packaging.
- Runtime hashes and both ZIP checksum manifests verified during packaging.

Native Windows interface testing and live funded-wallet rescans were not performed in this build environment. An accepted rescan request does not mean scanning has completed.
