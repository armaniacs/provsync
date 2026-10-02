---
title: provsync
---

<div class="tx-hero" markdown>

<div class="tx-hero__content" markdown>

# provsync

**Sync provider entries across your LLM tools into one central canonical config.**

provsync imports from a tool into the central config with `pull`, and reflects the central config back with `push`. Writes are preceded by an automatic backup, and `undo` restores.

[Read the docs](usage.en.md){ .md-button .md-button--primary }
[Open on GitHub](https://github.com/armaniacs/provsync){ .md-button }

</div>

</div>

<div class="grid cards" markdown>

- :material-shield-check-outline:{ .lg .middle } __Rewrite safely__

    ---

    Preview by default. All affected files are backed up automatically just before every write, and `undo` / redo bring them back

- :material-database-outline:{ .lg .middle } __One canonical config__

    ---

    Nothing is lost across pull → push round trips. `aliases` convert model IDs per tool automatically

- :material-stethoscope:{ .lg .middle } __Notice before it breaks__

    ---

    `doctor` diagnoses the environment and `check` verifies API reachability

</div>

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

Go 1.25.14 or later.

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

## Next Steps

- [Usage](usage.en.md) — quick start and feature details
- [CLI Reference](cli.en.md) — commands, flags, and exit codes
- [Backups and Undo](backup.en.md) — restoring and managing history

If this documentation doesn't answer your question, report it via [GitHub Issues](https://github.com/armaniacs/provsync/issues). Improvement proposals are accepted along the [CONTRIBUTING](https://github.com/armaniacs/provsync/blob/main/CONTRIBUTING.md) guide.
