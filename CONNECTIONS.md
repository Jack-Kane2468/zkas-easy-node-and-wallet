# Connect a wallet, application or development tool

Copy the desired address from Overview. Different applications use different settings: enter the address in that application's endpoint/server/base-URL field or configuration. No particular environment variable name is a ZKas-wide standard imposed by this manager.

- Wallet REST API: `http://127.0.0.1:8501` by default. Append documented paths such as `/api/status`. The copied value is the base URL, without an invented `/daemon` suffix.
- Node gRPC: `127.0.0.1:16810` by default. Use the ZKas protobuf API, not HTTP wallet paths.
- JSON wRPC: `ws://127.0.0.1:18810`. Use a compatible wRPC client.

`127.0.0.1` always refers to the computer making the connection. A container, WSL environment or another PC has a different localhost context. For remote wallet clients, use the explicit Sharing options.

The wallet daemon uses `--no-custodial`. It retains viewing/scan data and supports watch/prepare/submit operations. Spending keys remain with the client. A public watch-only operator can observe wallet information provided through viewing keys; watch-only does not mean the operator learns nothing.

Wallet operations retain the upstream per-wallet `X-Wallet-Token` header. Personal HTTPS also requires `Authorization: Bearer <access token>` from the separate copy button. Public HTTPS/onion does not remove per-wallet authentication. Personal onion authorization is handled by the client's Tor process before the API connection is made.

This version's remote gateway exposes wallet REST only. Node gRPC, JSON wRPC, bridge statistics, administrative routes and custodial/seed operations stay unshared. Browser CORS use is disabled; use native clients. Additional upstream API endpoints are denied until explicitly supported.

Existing programs with custom environment variable names can continue using them. This manager changes no program's configuration, database or keys. The earlier application-specific helper is omitted from this general distribution.

Upstream references:
- https://github.com/firecash/zkas-rusty/blob/zkas-v1.0.9/docs/WALLETD.md
- https://github.com/firecash/zkas-rusty/blob/zkas-v1.0.9/docs/NON_CUSTODIAL_WALLET.md
