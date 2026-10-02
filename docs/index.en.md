# provsync

**Sync provider entries across your LLM tools into one central canonical config.**

provsync is a CLI that syncs the `provider` entries your LLM coding tools each keep in their own format, using a central canonical config as the single source of truth. `pull` imports from a tool into the central config; `push` reflects the central config back into a tool. Built on the Go standard library only, with no external dependencies.

## Safety Design

- Preview-only by default. Files change only with `--write`.
- Every write is preceded by an automatic timestamped backup of all affected files as a single operation.
- `undo` records the pre-restore state as a new operation, so undo itself can be undone (redoable).
- Preview, `diff`, apply, and backup are all generated from the same Plan. What you see in the diff is exactly what gets written — and what undo restores.
- Byte-identical writes are skipped (idempotent). If nothing changes, you just get `変更はありません` ("no changes").

## Supported Tools

macOS / Linux only. Windows is unsupported.

| Tool | Config file | Format |
|---|---|---|
| kilocode (alias `kilo`) | `~/.config/kilo/kilo.jsonc` | JSONC (line comments, trailing commas) |
| opencode | `~/.config/opencode/opencode.json` | JSON |

- Central config: `~/.config/provsync/config.json`
- State (backups and history): `~/.local/state/provsync/`
- `XDG_CONFIG_HOME` / `XDG_STATE_HOME` are respected.

## Installation

Go 1.25.14 or later. macOS / Linux only (Windows is unsupported).

```bash
go install github.com/armaniacs/provsync@latest
```

Without the Go toolchain, download a prebuilt binary from Releases.

```bash
# Example for macOS (Apple Silicon)
curl -sLO https://github.com/armaniacs/provsync/releases/latest/download/provsync_<ver>_darwin_arm64.tar.gz
tar xzf provsync_*_darwin_arm64.tar.gz
sudo mv provsync /usr/local/bin/
```

Or build from source:

```bash
git clone https://github.com/armaniacs/provsync.git
cd provsync
make build    # bin/provsync
```

## Getting Started

Run `provsync init` to create the central config the first time.

```console
# Specify the source tool (preview first)
$ provsync init kilocode

# Create the central config with --write
$ provsync init kilocode --write

# Check the sync status, then reflect into other tools
$ provsync status
$ provsync push opencode --write
```

With the tool omitted, provsync detects tools whose config files exist. With multiple candidates, specify the tool with `provsync init <tool>`. An existing central config is never overwritten; use `pull` for everyday updates.

Next steps:

- [Usage](usage.md) — quick start and feature details
- [CLI Reference](cli.md) — commands, flags, and exit codes
- [Backups and Undo](backup.md) — restoring and managing history
