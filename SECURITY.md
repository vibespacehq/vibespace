# Security

## Reporting a vulnerability

If you find a security issue, please email **security@vibespace.build** instead of opening a public issue.

Include:

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Any potential impact you've identified

We'll acknowledge receipt within 48 hours and aim to provide a fix or mitigation plan within 7 days.

## Scope

Things we care about:

- Container escapes or privilege escalation
- WireGuard tunnel vulnerabilities
- Token forgery or replay attacks
- Credential leakage
- Unauthorized access to agent containers or the management API
- Path traversal via mounts or exec

Things that are out of scope:

- Denial of service against a local-only cluster (you're attacking yourself)
- Issues requiring physical access to the machine
- Social engineering
