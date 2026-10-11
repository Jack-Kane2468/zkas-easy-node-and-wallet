# 0.9.9-preview — Mining bridge updates and ZKrise wallet support

Each Mining page now has **Check for bridge updates…**, with installed version information. Kaspa and merged bridges check their official GitHub stable releases; the original ZKas-only bridge follows the installed ZKas core release. Downloads are verified before a running bridge is stopped. An updated running bridge restarts with saved addresses, worker names, ports and LAN settings. Nodes and wallets continue running during a bridge-only update.

New merged bridge installations use solo-dual-mode v1.0.12. The bundled official wallet signer is updated to zkas-wallet v1.0.47, and OVK viewing supports the named-anchor bundle format introduced by ZKas core v1.0.11. Merged-mining payout field order is unchanged: ZKas address as username (optional dot-suffixed worker), Kaspa address as password.

For ZKrise, update the manager, then use **Overview → Check node updates** for the ZKas node and enabled wallet backend, then **Mining → Check for bridge updates…** for the bridge in use. Updating the manager alone does not update those separately installed components.

Portable tests, offline wallet derivation/viewing vectors, Windows cross-build/static checks and package verification pass. Native Windows operation, physical miners and live payments were not tested in the build environment. See VALIDATION.md.
