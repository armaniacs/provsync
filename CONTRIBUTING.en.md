[日本語](./CONTRIBUTING.md) | English

# CONTRIBUTING

How to contribute to provsync.

## Getting Started

Go 1.25.14 or later is required.

```bash
git clone https://github.com/armaniacs/provsync.git
cd provsync
make check    # fmt-check → vet → test. Always run before finishing
make build    # bin/provsync
make lint     # staticcheck
make vuln     # govulncheck
make test-race
make fuzz     # short fuzz test for StripJSONC
```

Changes that do not pass `make check` are not merged. CI runs the same commands (`.github/workflows/ci.yml`).

## Commit Conventions

- Commit messages use English Conventional Commits (`feat:` / `fix:` / `docs:` / `test:` / `chore:`).
- Docs and code comments are in Japanese. CLI messages go through the `internal/i18n` catalog (en is authoritative, the default and fallback; ja is an additional language (`ja*` locales). Add new strings to both catalogs). Identifiers are in English.
- Catalog IDs are dot-separated `<type>.<area>.<what>`. `<type>` is `err` / `warn` / `msg` / `label`, `<area>` is `cli` / `store` / `tui`, etc., and `<what>` is camelCase (`err.tui.interactive`). Help and flags are `help.<cmd>` / `flag.<name>`, usage is `usage.<part>` and `usage.summary.<cmd>`. Drift display is `status.op.<op>` (`op` is the fixed set `not-in-tool` / `not-in-central` / `drift`).
- Comments explain "why" only (not "what"; no change history or issue numbers).
- Do not use `git add -A` / `git add .`; specify intended files individually.

## CHANGELOG

User-visible changes go into the `[Unreleased]` section of `CHANGELOG.md`. Keep a Changelog / SemVer.

## Secret Handling Principles

- Do not print secret values (`apiKey` / `token`, etc.) into output, logs, error messages, or test expectations (dummy test strings like `sk-test-...` excepted).
- Secret detection is centralized in `internal/secret`, shared by pull and the output layer — never duplicated.
- The central config holds only `apiKeyEnv` (an environment variable name); values are never mediated.
- File writes go through `fsutil.WriteFileAtomic` (temp file + rename).
- Never compute "what changes" separately in `internal/cli`; build a `plan.Plan` and reuse it.

## Adding a New Tool Adapter

provsync's value grows with the set of supported tools. Add a new tool as follows:

1. Create `internal/adapter/<tool>.go` implementing the `Adapter` interface (`Name` / `Path` / `Pull` / `Push` / `Project`). Keep it a thin layer that calls the shared helpers (`pullDocument` / `pushDocument` / `projectProviders`).
   - `Pull` must warn on and drop secret-like keys (per `internal/secret`); the shared part handles this.
   - `Push` must preserve providers outside the managed set and non-provider settings; the shared part handles this.
   - When a tool holds in-file secret keys, `carryOverSecrets` carries the values over on push to the same tool.
2. Add it to the `switch` in `adapter.Get` and to `Names()`.
3. Confirm the contract test (`internal/adapter/contract_test.go`) runs automatically. Contracts:
   - pull → push output is idempotent
   - in-file secret key values are preserved
   - tool-specific unknown fields are preserved
   - secret values never leak into the canonical form
4. When CLI help/completion need updating, update the registry (`commandRegistry`) in `internal/cli/help.go` and the supported-tools section of the README. Dispatch, help, usage, and completion are all generated from this registry.

## Expected Adapter Behavior (Covered by Contract Tests)

| Behavior | Tests |
|---|---|
| pull → push is idempotent | `TestAdapterContract` / `TestPushIsIdempotentPerTool` |
| Preserves in-file secret keys | `TestAdapterContract` |
| Preserves unknown fields | `TestAdapterContract` / `TestPullPushRoundTripsUnknownFields` |
| No secret leakage into the canonical form | `TestAdapterContract` |
| Stable push output | `TestPushGolden` |
