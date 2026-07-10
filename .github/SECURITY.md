# Security

## Secret scanning

This project uses a layered approach to prevent credential leaks:

### Pre-commit (every commit)
**[Gitleaks](https://github.com/zricethezav/gitleaks)** scans staged changes in ~700ms and blocks commits containing known secret patterns (API keys, tokens, private keys). This catches the 95% case before secrets ever enter git history.

### CI gate (every push/PR)
**Gitleaks** runs in CI with full history (`fetch-depth: 0`) and SARIF output. Acts as a second line of defense if the pre-commit hook is bypassed.

### GitHub Secret Scanning (automatic)
For public repos, [GitHub Secret Scanning](https://docs.github.com/en/code-security/secret-scanning/about-secret-scanning) is free and runs automatically on every push. It detects 300+ partner token patterns and notifies credential issuers privately -- no findings appear in public CI logs.

### Local deep scan (on demand)
**[TruffleHog](https://github.com/trufflesecurity/trufflehog)** is available in the devenv shell for personal security audits. It verifies detected credentials against live APIs (confirms whether a leaked key is still active) and scans beyond git (Docker images, filesystem). **Do not run TruffleHog in public CI** -- its output can contain credential fragments and the verification step sends detected strings to external APIs.

```bash
# Local verified scan of recent history
trufflehog git file://$(pwd) --since-commit HEAD~500 --only-verified

# Filesystem scan (non-git files)
trufflehog filesystem .
```

## Known limitations

**WebSocket `/responses` endpoint is not covered by model policy.** The policy engine blocks models based on the request body's `model` field. WebSocket upgrade requests have no body, so the model is only known after the connection is established (inside `response.create` frames). Until frame-level policy inspection is implemented, `/responses` traffic bypasses the model allow/block policy.

If you discover a security issue, please do **not** open a public issue. Instead, report it via [GitHub's private vulnerability reporting](https://github.com/koopycat/copilot-monitor/security/advisories/new).
