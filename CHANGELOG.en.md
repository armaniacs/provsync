# Changelog

Records notable changes to this project.
Format follows [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/), versioning follows [Semantic Versioning](https://semver.org/lang/ja/).

## [Unreleased]

## [0.3.1] - 2026-10-03

### Fixed

- Fixed a race where `provsync-tui` wrote apply-command completion results to the model outside the event loop. Results are now returned as Msg to Update, with screen transitions (confirm→done→quit, confirm→list) pinned by tests. No external behavior change.

### Changed

- Changed the default CLI/TUI message language from Japanese to English (**breaking change in default display**). Japanese is used only for locales starting with `ja` (resolved in `PROVSYNC_LANG` / `LC_ALL` / `LC_MESSAGES` / `LANG` priority), and unsupported or empty locales fall back to English. Non-localized rendering via `Message.Error()` is also English. Set `PROVSYNC_LANG=ja` to keep using Japanese as before.

### Added (CI)

- Added `all` (builds both CLI and TUI) / `tui-test` (TUI module tests) / `test-all` (both core and TUI module tests) to the Makefile.

## [0.3.0] - 2026-10-03

Japanese-English localization of CLI/TUI messages, plus a documentation site.

### Added

- Localized CLI messages in Japanese and English (via the new `internal/i18n` catalog). Default remains Japanese. Language is resolved in `PROVSYNC_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` priority; locales starting with `en` produce English, unsupported or empty locales fall back to Japanese. `provsync-tui` shares the same catalog (via the `replace` directive in `tui/go.mod`) so child-process and display languages never mix. `status --json` keeps its key structure across locales, but `drift` lines and `warnings` values follow the locale. Use `driftEntries` for machine-readable drift as before.

- Added a documentation site (MkDocs Material + mkdocs-static-i18n). Pushes to main auto-deploy via GitHub Actions (`https://armaniacs.github.io/provsync/`). Japanese is the default, with an English version via language switcher. Covers: home / usage / CLI reference / backup and undo / security / developer guide / CHANGELOG. `make docs-serve` for local preview; dependencies pinned in `requirements.txt`.
- Show the version with `provsync --version` / `provsync version`. Resolution order: ldflags-injected value at build time, module version from `go install`, then `dev`. `make build` injects the `git describe` result via ldflags.
- Show the version at the top of bare-invocation and `--help` output.
- Show central config, tool config, and backup store paths with existence (uncreated) in bare-invocation and `--help` output. When the central config does not exist yet, show creation guidance.
- Added `driftEntries` (structured drift) to `status --json` additively. `op` is the fixed set `not-in-tool` / `not-in-central` / `drift`. Existing `drift` strings and text output are unchanged. `provsync-tui` prefers driftEntries and falls back to reverse-parsing strings for older CLI responses.
- Mask values of secret-like keys (`apiKey` / `token`, etc.) as `********` by default in `diff` output. `--show-secrets` shows raw values (with a warning). Written file contents are never masked. Secret-key detection is consolidated into shared logic (`internal/secret`) with pull.
- `init [tool]` command. Creates the central config with the same Plan as `pull` for first-time setup. Never overwrites an existing central config. When the tool is omitted, detects tools whose config files exist and auto-selects when there is exactly one candidate. When secrets are detected, warns with migration guidance to `apiKeyEnv`.

### Added (CLI UX)

- Added per-subcommand help (`provsync <command> --help`) and `completion bash|zsh|fish`.
- Systematized exit codes: `0` success / `1` runtime error / `2` usage error. Usage errors are distinguished by the `UsageError` type.
- Split normal output to stdout, warnings and flag-parse errors to stderr (stdout / stderr injectable via `RunWith`).

### Added (backup)

- Made backup retention configurable via the `PROVSYNC_KEEP` environment variable (default stays 20).
- Clean up history with `provsync undo --prune --keep <n>`. Keeps the newest n entries, deletes the rest, and prints the deleted count.
- Newly created files are created with 0600, new directories with 0700 (existing file permissions are inherited). `status` suggests `chmod` when central config or state-directory permissions are too open.

### Added (release)

- Release automation (`.goreleaser.yaml` and `.github/workflows/release.yml`). A tag push attaches darwin / linux (amd64, arm64) tar.gz archives and checksums to the GitHub Release. Fails when the CHANGELOG has no section for the version.
- Documented binary downloads in the README installation steps. Supported environments explicitly list macOS / Linux (Windows unsupported).

### Fixed

- When the write target is a symbolic link, the link itself was replaced with a regular file. Now updates the link target atomically while preserving the link. Broken links are a clear error. `list` shows the link target.

### Added (platform)

- Windows is explicitly unsupported at startup (`cli.Supported`; exit code 1 on Windows).
- Added regression tests for XDG resolution (`XDG_CONFIG_HOME` / `XDG_STATE_HOME`) in `adapter.NewRoot`.

### Added (JSON)

- Added `--json` to `list` / `status` / `diff` (includes `schemaVersion: 1`). With `--json`, stdout carries JSON only and warnings go to stderr.
- Added `--exit-code` to `status`. Exits with code 3 when drift exists (distinguished by the `ExitError` type).

### Added (aliases)

- With `aliases` (common model name → tool → model ID) in the central config, `push` auto-converts model names to per-tool IDs. Aliases without a mapping for the target tool pass through with a warning, or error with `--strict`. Pull never rewrites IDs, and `aliases` survive pull. Schema is additive-only; version stays 1.

### Added (tests)

- Added `FuzzStripJSONC` (standard `testing.F`). Fuzz-verifies that comment markers and trailing commas inside string literals are never broken. Runnable briefly with `make fuzz`.
- Added pull → push round-trip idempotence tests for all adapters, plus golden tests for push output (update with `-args -update`).

### Added (lock)

- Introduced an exclusive lock (`syscall.Flock`) on the state directory around the `--write` write window and the `undo` restore window. Later processes wait; timeout (10 seconds) exits with an error. Preview, `diff`, and `status` take no lock. No PID-file scheme; the OS releases the lock automatically on process exit.

### Added (doctor)

- `provsync doctor` command. Prints a diagnosis of path resolution, existence and syntax of the central and tool configs, `apiKeyEnv` environment-variable presence, and file permissions. No network access, writes no files. Exits with code 1 when any NG item exists. Never prints secret values.

### Added (docs)

- Added CONTRIBUTING.md (for contributors). Covers development workflow, commit conventions, secret-handling principles, and an adapter guide for new tools.
- Added a shared contract test (`TestAdapterContract`) applied to all adapters. Verifies round-trip idempotence, preservation of in-file secret keys, preservation of unknown fields, and that secrets never leak into canonical form.

### Added (check)

- `provsync check` command. Sends an authenticated GET to `<baseURL>/models` for each provider in the central config and classifies reachability (`[OK]` / `[auth failed]` / `[bad response]` / `[unreachable]` / `[skip]`). 5-second timeout. This is the only command that uses the network; key values are never printed.

### Added (routes)

- With `routes` (alias name → prioritized provider keys) in the central config, `push` selects a usable route (provider with `apiKeyEnv` set or requiring none) and drops unselected route providers from rendering. The check uses environment-variable presence only; no network access. When no route is usable, warns and leaves things unchanged. `routes` survive pull. Schema is additive-only; version stays 1.

### Added (tui)

- `provsync-tui` (optional). A TUI for selecting providers and target tools with checkboxes and applying after a confirmation screen. Isolated as an independent Go module under `tui/` so the core `go.mod` gains no external dependencies. The TUI invokes the `provsync` binary as a child process and never computes changes itself. Exits with an error on non-interactive terminals. The isolation decision is recorded in `docs/superpowers/specs/2026-10-03-tui-isolation-decision.md`.

### Added (CI)

- Added CI (`.github/workflows/ci.yml`). Runs `make check` / `make test-race` / `make lint` / `make vuln` on a macOS / Linux matrix for pushes to main and PRs.
- Added `lint` (staticcheck) / `vuln` (govulncheck) / `test-race` to the Makefile. Lint tools are fetched via `go run pkg@version` without adding dependencies to `go.mod`.

### Docs

- README: updated the `diff` example to match actual behavior (secret masking). Added the `--show-secrets` flag.
- Added SECURITY.md (security policy). Covers private vulnerability reporting and scope.

## [0.2.1] - 2026-10-02

Minor fixes and documentation updates.

### Fixed

- A provider with empty `models` kept showing as "updated (models)" in the `diff` preview and as drift in `status` after pull. Saving with empty `models` omitted caused a nil-vs-empty comparison difference; a false drift with no file change.
- Fixed doubled slashes (`a//home/...`) in `diff` headers when paths are absolute.

### Docs

- README: fixed examples to match actual behavior (`status` secret-detection warning line, `diff` header notation). Added notes on secret display in `diff` / `status` to the secret-handling section. Documented the `version` field, the `--help` description, and the required Go version (1.25.14).
- AGENTS.md: fixed a stale flag list (`--write` → `--backup`).
- Updated the design doc (2026-10-02) as-built (marker operations, undo warnings, `models` equivalence, etc.).
- Added a "superseded" note to the old design and implementation plan (2026-10-01).

## [0.2.0] - 2026-10-02

Redesign for multi-tool sync.

### Changed

- Switched the CLI to subcommands (`list` / `status` / `pull` / `push` / `sync` / `diff` / `undo`). Removed the old flags (`--source` / `--target` / `--providers` / `--backup`) (**breaking change**).
- Moved to a design with the central canonical config `~/.config/provsync/config.json` as the single source of truth.
- Normalized known provider fields; tool-specific unknown fields are kept in `Extras` and round-tripped.
- `--provider` accepts comma-separated and repeatable values. Backup disabling changed to `--no-backup`.
- Compare semantic diff on the tool-visible projection (adapter `Project`), fixing false positives in `status` and preview from fields the tool never renders (`apiKeyEnv`, etc.).
- Pull preserves other tools' extras namespaces and `version` from the existing central config.
- `status` treats a broken central config as an error (distinct from uncreated).

### Added

- Preview / diff / apply with `Plan` as the single source of truth.
- Semantic diff + unified diff display (`diff`).
- Timestamped backups, manifest, and `undo` (undo itself is undoable). Undo prints the operation ID for redo.
- Adapter IF (kilocode / opencode, alias `kilo`).
- Secrets-are-never-mediated policy (`apiKeyEnv` reference only). Push preserves existing secret fields (`options.apiKey`, etc.) in the target tool config.
- Writes with `--no-backup` are recorded as marker operations in history, and later `undo` warns about them.

### Removed

- Removed the one-shot `kilo.jsonc` → `opencode.json` porting CLI and the old `main_test.go`.

### Migration guide (from 0.1.0)

- The old `provsync --source X --target Y --write` equivalent is `provsync sync --from kilocode --to opencode --write`.
- The old `--providers a,b` equivalent is `--provider a --provider b` (comma-separated also works).
- The old `--backup=false` equivalent is `--no-backup`. Without it, a backup is recorded automatically before writing.
- `opencode.json.bak` files created by 0.1.0 are not included in the new `undo` history (manual restore only).

## [0.1.0] - 2026-10-01

Initial release.

### Added

- `provsync` CLI. Ports provider entries from `~/.config/kilo/kilo.jsonc` to `~/.config/opencode/opencode.json`.
- JSONC (line comments, trailing commas) preprocessing `StripJSONC`.
- `Merge` that ports providers wholesale by overwrite (keeps out-of-scope keys).
- Preview by default. Updates files with `--write`.
- Backup (`.bak`) and atomic replacement on write.
- Flags: `--source` / `--target` / `--write` / `--backup` / `--providers`.
- Unit tests and integration tests for `run()`.
- `Makefile` (build / test / check / write, etc.).
- `README.md`.
