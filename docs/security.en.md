# Security

## Secret Handling (Design)

- The central config holds only `apiKeyEnv` (an environment variable name). Actual keys are never mediated.
- On `pull`, secret-like fields (`apiKey` / `api_key` / `token` / `secret` / `password` / `accessToken` / `access_token`, including inside `options`) are dropped with a warning.
- On `push`, secret fields that already exist in the target tool config are preserved as-is (never deleted, never leaked into the central config).
- `apiKeyEnv` is never rendered for opencode (its keys live in `auth.json`).
- `diff` output masks values of secret-like keys as `********` by default. The written file content is never masked (display only).
- `status` / `list` show key-name warnings only; values are never printed.
- Backups contain the pre-write file content, which may include in-file secrets.

## Network Scope

The only command that talks to the network is `provsync check` (API reachability per provider). Other commands never access anything beyond the synced config files and the state directory.

## Reporting a Vulnerability

If you find a vulnerability, do not open a public issue; report it privately via GitHub's "Report a vulnerability" (Security tab > Advisories).

- Include reproduction steps, affected versions, and the expected impact.
- Do not include actual API keys or other secrets.
- Receipts are acknowledged within 7 days.
- Fixes target the latest minor series only.
- The report is published after the fixed release, with the reporter's consent.

## Scope

The following are in scope:

- Secrets (`apiKey`, `token`, etc.) being written somewhere other than output, logs, the central config, or backups — or displayed unintentionally
- Information leaks caused by backup or state directory permissions
- Config file corruption, or issues that could overwrite unintended files
