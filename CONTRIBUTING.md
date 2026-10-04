# Contributing

Open a bug report or discuss a feature before a large change. Describe the affected version, expected behavior and a minimal reproduction. Use disposable keys and sanitize logs; never send a real recovery phrase or viewing key.

Use BUILDING.md for a fresh-checkout build. Go changes should be gofmt-formatted and pass tests/vet. Changes to startup, stopping, installation or update behavior must preserve existing data and include focused lifecycle coverage. Wallet/signing changes need test vectors and a clear review of amounts, fees and what is signed. Do not replace upstream cryptographic primitives with custom algorithms.

Keep the default interface understandable to non-developers. Show missing data as unavailable, and explain destructive actions before performing them.

Submit pull requests with the problem, resulting behavior, validation performed and remaining test gaps. Contributions to manager code are under MIT; viewing-helper changes follow its ISC license. Preserve third-party notices.
