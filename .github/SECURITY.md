# Security Policy

## Supported Versions

| Version | Status |
| --- | --- |
| Latest Release | ✅ Supported |
| Development (main) | ⚠️ Bug fixes only |

## Reporting a Vulnerability

If you discover a security vulnerability, **please do not** report it via a public issue.

Instead, please report it privately through one of the following:

1. Use GitHub's [Private Vulnerability Reporting](https://github.com/AimAI-Labs/mihosh/security/advisories/new)
2. Or contact the repository maintainers directly

Please include in your report:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

We will respond as soon as possible and publicly acknowledge contributors after the fix is released.

## Security Considerations

- Mihosh communicates with the Mihomo core via HTTP API — keep your API Secret safe
- The config file (`~/.mihosh/config.yaml`) contains API credentials — do not share it publicly
- When submitting Bug Reports, always redact sensitive information such as `secret` and `api_address`
