# Development

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
- Docs, CLI messages, and code comments are in Japanese. Identifiers are in English.
- Comments explain "why" only (not "what"; no change history or issue numbers).
- Do not use `git add -A` / `git add .`; add intended files individually.

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
2. Add it to the `switch` in `adapter.Get` and to `Names()`.
3. Confirm the contract test (`internal/adapter/contract_test.go`) runs automatically. Contracts: pull → push idempotency, in-file secret preservation, unknown-field preservation, and no secret leakage into the canonical form.
4. When CLI help/completion need updating, edit the registry in `internal/cli/help.go` and the "Supported Tools" section of the README.

## Package Layout

- `main.go` — dispatch; `cli.RunWith` is the testable entrypoint
- `internal/cli` — subcommands (`cli.go` (dispatch and common write path), `sync.go` (sync commands and pipelines), `status.go` (list and status), `history.go` (diff and undo), `doctor.go` (diagnosis), `check.go` (reachability), `help.go` (help and completion registry), `render.go` (shared rendering), `errors.go` (usage errors), `platform.go` (supported OS))
- `internal/model` — canonical representation
- `internal/adapter` — kilocode / opencode conversion
- `internal/store` — central config
- `internal/syncer` — merge, aliases, route selection
- `internal/jsonc` — JSONC preprocessing
- `internal/plan` — change plan and semantic diff
- `internal/diff` — unified diff
- `internal/backup` — backup and restore
- `internal/fsutil` — atomic write and JSON formatting

## Related Documents

- [Design document](https://github.com/armaniacs/provsync/blob/main/docs/superpowers/specs/2026-10-02-provsync-multi-tool-sync-design.md)
- [CONTRIBUTING.md](https://github.com/armaniacs/provsync/blob/main/CONTRIBUTING.md) (contributor guide: adding an adapter)

## Updating This Site

This site is generated with MkDocs Material and deployed automatically on push to main.

```bash
make docs-serve    # local preview (http://localhost:8000)
make docs          # build into site/
```

Dependencies are pinned in `requirements.txt` (`pip install -r requirements.txt`).
