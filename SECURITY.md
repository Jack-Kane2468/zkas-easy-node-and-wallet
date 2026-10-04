# Security and privacy

This is an unsigned, unaudited community preview. A passing build or dependency scan is not an audit and does not establish that real payments or public hosting are safe. Known validation limits are listed in VALIDATION.md.

## Report privately

For a suspected vulnerability, use this repository's **Security → Report a vulnerability** once the maintainer enables GitHub private reporting. Do not post exploit details or private wallet information in a public issue. If private reporting is unavailable, open a content-free request asking the maintainer to enable it; do not attach secrets or an exploit.

Include affected version, Windows version, a minimal reproduction with disposable test data, impact and any proposed fix. There is no guaranteed response time. The latest preview is the maintenance target; old builds are not promised security updates.

## What to keep private

Never upload your recovery phrase, spending key, vault, local application folder, API credentials, TLS private key or Tor identity. FVKs/OVKs and wallet addresses can disclose financial activity. Remove them and public IPs from screenshots and logs before sharing. Repository fixtures are public test vectors and must never receive funds.

## Trust boundaries

The GUI signs using a checksum-pinned local runtime. Runtime hashes detect unexpected bytes; they are not digital signatures or protection from a compromised operating system. The encrypted vault protects stored data; an unlocked wallet and clipboard remain accessible to malware running as you.

The node, wallet daemon, Tor and mining bridge are separate upstream programs. Their releases and your PC's security remain part of the trust model. Node/bridge downloads use upstream release checksums; the project does not independently certify those binaries.

Local APIs are loopback-bound by default. Sharing must be deliberately enabled. Public registration can consume disk/CPU; gateway limits do not replace external capacity management, monitoring and abuse protection. Tor identity reuse and HTTPS certificates are retained with user data on uninstall.

## Maintainer release practice

Run the checks in RELEASE-CHECKLIST.md, enable private vulnerability reporting and secret scanning where available, and review dependency alerts. Do not label the preview audited, production-certified or fully tested. Sign releases when a signing identity is available; never put signing secrets in repository files.
