# CLI Reference

## Commands

| Command | Description |
|---|---|
| `list` | List supported tools and their config paths |
| `status [tool...]` | Show sync status between tools and the central config |
| `init [tool]` | First-time setup (creates the central config). Never overwrites an existing one |
| `pull <tool>` | Import a tool config into the central config |
| `push <tool>` | Reflect the central config into a tool config |
| `sync --from <a> --to <b>` | Pull from a and push to b (`--from` optional: uses the central config as-is) |
| `diff <from> <to>` | Show what applying from to to would change (semantic + unified diff) |
| `undo [id]` | Restore the last (or given) operation (`--list` for history, `--prune --keep <n>` to clean up) |
| `doctor` | Diagnose the environment (existence, syntax, `apiKeyEnv`, permissions; no network) |
| `check` | Check API reachability per provider (the only command that talks to the network) |
| `completion <shell>` | Print a completion script for bash / zsh / fish |
| `--version` / `version` | Show the version |

## Flags

| Flag | Default | Description |
|---|---|---|
| `--write` | `false` | Write changes to files (preview by default) |
| `--provider <p>` | all providers | Restrict to specific providers (comma-separated, repeatable) |
| `--no-backup` | `false` | Skip backup recording (discouraged). The write is still recorded as a marker operation |
| `--show-secrets` | `false` | Show raw secret values in `diff` output (discouraged). A warning is printed |
| `--root <dir>` | `$HOME` | Override the base directory for path resolution (testing) |
| `--from` / `--to` | — | Source / target tools for `sync` |
| `--list` | `false` | Show `undo` history |
| `--prune` / `--keep <n>` | `false` / `20` | Clean up `undo` history / ops to keep |
| `--json` | `false` | Output `list` / `status` / `diff` as JSON |
| `--exit-code` | `false` | Exit with code 3 when `status` detects drift |
| `--strict` | `false` | Fail when model names have no alias mapping (push) |
| `--version` | `false` | Show the version |
| `--help` / `-h` | `false` | Show help |

- Flags may appear after positional arguments (e.g. `provsync push opencode --write`).
- Per-command help is available via `provsync <command> --help`.

## Exit Codes

| Exit code | Meaning |
|---|---|
| `0` | Success (no drift) |
| `1` | Runtime error (config load failure, etc.) |
| `2` | Usage error (unknown command, wrong argument count, etc.) |
| `3` | Drift detected (`status --exit-code` only) |

Errors and warnings go to stderr; normal output to stdout.

## Machine-Readable Output (--json)

`list` / `status` / `diff` accept `--json`; stdout then carries JSON only (warnings go to stderr). The schema carries `schemaVersion: 1`.

```json
{
  "schemaVersion": 1,
  "central": { "path": "~/.config/provsync/config.json", "exists": true, "providers": 2 },
  "tools": [
    {
      "name": "kilocode",
      "path": "~/.config/kilo/kilo.jsonc",
      "exists": true,
      "providers": 2,
      "warnings": ["secret-like field ..."],
      "drift": ["llm-01: not in tool"],
      "driftEntries": [{ "provider": "llm-01", "op": "not-in-tool" }]
    }
  ]
}
```

- `status --json`: the shape above; empty `drift` = no drift. `driftEntries` is the structured version of `drift` (additive); `op` is one of the fixed set `not-in-tool` / `not-in-central` / `drift`. Interactive tools (the TUI, etc.) should use `driftEntries` instead of parsing the human-readable strings.
- `list --json`: the same shape without `warnings` / `drift`.
- `diff --json`: `{"schemaVersion": 1, "changes": [{"tool", "path", "semantic": [...], "diff": "masked unified diff"}]}`.
- `status --exit-code`: exit code `3` when drift exists, `0` when synced.

## Shell Completion

```bash
# zsh
provsync completion zsh > "${fpath[1]}/_provsync"

# bash
source <(provsync completion bash)

# fish
provsync completion fish | source
```
