# Security policy

## Supported versions

Security fixes are applied to the latest tagged release and `main` while the project is pre-1.0.

## Reporting

Do not open a public issue for a vulnerability that could enable authorization bypass, token misuse, tenant crossing, secret disclosure, or admission-control bypass. Use GitHub private vulnerability reporting when enabled for the repository.

Include:

- affected commit/tag;
- sanitized reproduction;
- expected versus observed decision;
- impact and tenant boundary;
- whether a dependency must fail or be controlled;
- suggested mitigation if known.

Do not include live credentials, personal data, or production identifiers.

## Local demo credentials

Credentials under `compose.yaml` and `demo/keycloak/realm-export.json` are intentionally local-only examples. They are not suitable for any shared or internet-accessible environment.

## Security assumptions

- identity headers are trusted only after a validated-token boundary;
- TLS/mTLS and secret-management are deployment responsibilities;
- remote policy/model revisions are pinned;
- audit data is sensitive;
- fail-closed behavior is the baseline.
