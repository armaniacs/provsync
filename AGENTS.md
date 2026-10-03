# AGENTS.md

CLI that syncs `provider` entries across LLM tool configs (kilocode / opencode) using a
central canonical config as the single source of truth. Parse/convert is per-tool adapters;
the central config lives at `~/.config/provsync/config.json`.

## Commands

- `make check` — fmt-check → vet → test; run this before finishing. Order matters (fmt-check fails fast on unformatted code).
- `make test` / `make test-v` — `go test ./...`
- `make test-race` — `go test -race -cover ./...`
- `make lint` — staticcheck (downloaded via `go run`, does not touch go.mod)
- `make vuln` — govulncheck (downloaded via `go run`, does not touch go.mod)
- `make build` — outputs to `bin/provsync` (gitignored)
- `make fmt` — `gofmt -w .`

CI runs `make check`, `make test-race`, `make lint`, and `make vuln` on push to main and on PRs (`.github/workflows/ci.yml`, macOS / Linux matrix). Keep local and CI commands identical via make. No module dependencies in `go.mod`: standard library only; linters are pulled at runtime with `go run pkg@version`. Do not add a module dependency without reason.

## Gotchas

- Default is **preview only**; files change only with `--write`. There is no one-shot flag mode anymore — the old `--source/--target/--providers/--backup` CLI was replaced by subcommands (`list`/`status`/`pull`/`push`/`sync`/`diff`/`undo`).
- All writes are backed up as timestamped operations under `~/.local/state/provsync/` (unless `--no-backup`), then applied via temp file + `os.Rename` (atomic). Preserve this.
- `undo` restores the last (or a given) operation, records the pre-restore state as a new operation (redoable), ignores `--write`, and never writes without a recorded backup path.
- `Plan` (`internal/plan`) is the single source of truth for preview, `diff`, apply, and backup. Do not compute "what changes" separately in `cli`; build a Plan and reuse it.
- Output JSON is fully re-serialized with 2-space indent and alphabetically sorted keys; provider entries not in the managed set are preserved as-is. Full re-serialization means original formatting/comments are lost (accepted; `diff` may show formatting churn).
- Secrets are never mediated. The central config holds only `apiKeyEnv` (an env var name). Pull warns and drops secret-like keys (`apiKey`, `token`, ...); push preserves the target tool's existing in-file secret keys instead of deleting them; `adapter.opencode` never renders `apiKeyEnv` (opencode stores keys in `auth.json`).
- JSONC handling lives in `internal/jsonc` and is applied by the kilocode adapter before `encoding/json`. Keep merge logic (`internal/syncer`) independent of JSONC handling.
- Unknown tool-specific fields are preserved under `model.Provider.Extras[<tool>]` and restored on push to the same tool.
- `status --json` keeps its key structure across locales, but the `drift` lines and `warnings` values are locale-linked. Clients that need machine-readable drift must use `driftEntries` (fixed `op` values), not the text.

## Layout

- `main.go` — dispatch only; `cli.Run(args, out)` is the testable entrypoint.
- `internal/cli/` — flag parsing (`reorder` allows flags after positional args) and command handlers.
- `internal/model/` — canonical `Config`/`Provider` (known fields + per-tool `Extras`).
- `internal/adapter/` — `Adapter` IF, registry/aliases, shared document decode/encode, kilocode/opencode.
- `internal/store/` — central config load/marshal/save.
- `internal/syncer/` — provider set merge and filter (pure).
- `internal/jsonc/` — `StripJSONC` (line comments + trailing commas).
- `internal/plan/` — `Plan`/`FileChange`/`ProviderChange` and provider diff.
- `internal/diff/` — unified diff renderer.
- `internal/backup/` — timestamped backups, manifest index, restore, retention.
- `internal/fsutil/` — atomic write and sorted JSON marshal.
- `internal/i18n/` — language resolution (`PROVSYNC_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > ja) and the ja/en message catalog. Internal packages return language-neutral `*i18n.Message` values; `cli.RunWith`/`main.go` localize at the boundary.

Integration tests for `cli.Run` use `--root <tmpdir>` and real files under `t.TempDir()`.

## Conventions

- Docs and code comments are in Japanese. CLI messages live in the `internal/i18n` catalog: ja is the source of truth and en mirrors it. Add every new user-facing string to both catalogs. Catalog IDs are dotted `<type>.<area>.<what>` (err/warn/msg/label + area, camelCase what); `help.<cmd>` / `flag.<name>` / `usage.<part>` for help and flags; `status.op.<op>` for the fixed drift op set.
- Commit messages use English Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- `CHANGELOG.md` follows Keep a Changelog / SemVer; update it for user-visible changes.
- Design/plan docs live in `docs/superpowers/{specs,plans}/`.