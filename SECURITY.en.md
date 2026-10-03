[日本語](./SECURITY.md) | English

# Security Policy

## Reporting

If you find a vulnerability, do not file a public issue. Report it privately via GitHub "Report a vulnerability" (Security tab > Advisories).

Include reproduction steps, affected versions, and expected impact. Do not include actual secrets such as API keys.

## Response Policy

- Acknowledgment within 7 days as a guideline.
- Fixes target only the latest minor version series.
- Details are disclosed after the fixed release, with the reporter's consent.

## Scope

provsync syncs `provider` entries in config files. The following are in scope for reports:

- Secrets (`apiKey`, `token`, etc.) written to, or unintentionally displayed from, places other than output, logs, the central config, or backups
- Information leaks caused by permissions on backup or state directories
- Issues leading to config file corruption or unintended file overwrites

## Secret Handling (Design)

- The central config holds only the environment variable name (`apiKeyEnv`), never secret values.
- Push preserves existing secrets in the target tool's file instead of deleting them.
- Backups store pre-write file contents (which may include in-file secrets).
