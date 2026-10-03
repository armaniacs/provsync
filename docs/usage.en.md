# Usage

The everyday sync flow and details of each feature.

!!! note
    CLI messages are English by default. Set `PROVSYNC_LANG` (or `LC_ALL` / `LC_MESSAGES` / `LANG`) to a locale starting with `ja` for Japanese output; unsupported locales fall back to English.

## Quick Start

```console
# Import kilocode providers into the central config (preview first)
$ provsync pull kilocode
central: /home/you/.config/provsync/config.json
  llm-01: added
  llm-02: added
(preview only; apply with --write)

# Apply with --write; a backup is recorded just before writing
$ provsync pull kilocode --write
central: /home/you/.config/provsync/config.json
  llm-01: added
  llm-02: added
backup: 20261002T093012-3fa1
wrote: /home/you/.config/provsync/config.json

# Reflect the central config into opencode
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  llm-01: added
  llm-02: added
backup: 20261002T093045-8c2d
wrote: /home/you/.config/opencode/opencode.json

# Re-writing the same state is idempotent
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  no change
no changes
```

## Check Sync Status

```console
$ provsync status
central config: /home/you/.config/provsync/config.json (2 providers)
kilocode  /home/you/.config/kilo/kilo.jsonc (2 providers)
  no drift
opencode  /home/you/.config/opencode/opencode.json (3 providers)
  warning: detected a secret-like field "options.apiKey"; secrets are never mediated, so it was not imported
  groq: not in central
```

Providers that exist only in a tool (`groq` above) show as "not in central". `push` never deletes unmanaged entries in the target tool config.

## Inspect Changes and Undo

```console
$ provsync diff kilocode opencode
opencode: /home/you/.config/opencode/opencode.json
  llm-01: added
  llm-02: added
--- a/home/you/.config/opencode/opencode.json
+++ b/home/you/.config/opencode/opencode.json
@@ -1,15 +1,44 @@
   ...

$ provsync undo
restored: 20261002T093045-8c2d (push opencode)
redo: provsync undo 20261002T100001-51b7
  restored: /home/you/.config/opencode/opencode.json
```

`undo` applies directly; it does not require `--write`.

## Central Config

Known fields (`name` / `npm` / `baseURL` / `apiKeyEnv` / `models`) are normalized, while tool-specific unknown fields are namespaced under `x.<tool>` and restored when pushing back to the same tool — nothing is lost across a pull/push round trip.

```json
{
  "providers": {
    "llm-01": {
      "apiKeyEnv": "LLM01_API_KEY",
      "baseURL": "https://llm-01.example.com/v1",
      "models": {
        "model-a": { "name": "Model A" },
        "model-b": { "name": "Model B" }
      },
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "x": {
        "kilocode": { "reasoning": true }
      }
    }
  },
  "version": 1
}
```

`version` is the central config format version (reserved for future migrations; currently `1`).

## Model Aliases

`aliases` in the central config maps a common model name to per-tool model IDs; `push` converts each tool's required ID format automatically.

```json
{
  "aliases": {
    "sonnet": {
      "kilocode": "claude-sonnet-4-5",
      "opencode": "anthropic/claude-sonnet-4-5"
    }
  },
  "providers": {
    "llm-01": {
      "models": { "sonnet": { "name": "Sonnet" } }
    }
  }
}
```

- Conversion happens only before push rendering. `pull` never rewrites IDs (no information loss across round trips).
- Model names not in `aliases` pass through unchanged.
- An alias without a mapping for the target tool passes through with a warning. `--strict` errors out and writes nothing.
- There is no built-in mapping table. The user-defined `aliases` are the source of truth and survive `pull`.

## Route Fallback (routes)

`routes` in the central config maps an alias name to a priority-ordered list of provider keys; on `push`, the first usable route (a provider whose `apiKeyEnv` is set, or that needs no env) is selected and unselected route providers are dropped from the render.

```json
{
  "routes": {
    "sonnet": ["anthropic", "openrouter"]
  },
  "providers": {
    "anthropic": { "apiKeyEnv": "ANTHROPIC_API_KEY", "models": { "sonnet": {} } },
    "openrouter": { "apiKeyEnv": "OPENROUTER_API_KEY", "models": { "sonnet": {} } }
  }
}
```

- Selection uses environment-variable presence only. No network access; values are never read or printed.
- When no route is usable, provsync warns and leaves the providers unchanged.
- A provider with an empty `apiKeyEnv` needs no env and is always usable.
- `routes` survive `pull`. Per-tool overrides are not supported (all tools share the routes).

## API Reachability (check)

`provsync check` sends an authenticated GET to `<baseURL>/models` for each provider in the central config and prints reachability.

- `check` is the only command that talks to the network. Other commands never access anything beyond the synced files.
- Key values are read from environment variables but never appear in output or logs. 401/403 shows as `[auth failed]`; timeouts and connection failures show as `[unreachable]`.
- Providers with an unset `apiKeyEnv`, environment variable, or `baseURL` are skipped without any request (`[skip]`).

## Environment Diagnosis (doctor)

`provsync doctor` lists a diagnosis of path resolution, existence and syntax of the central and tool configs, `apiKeyEnv` environment variables, and file permissions. It never talks to the network and never writes files. It exits with code 1 when any NG item is found.

```console
$ provsync doctor
[OK] path resolution
[OK] central config
[warning] apiKeyEnv llm-01: env var LLM01_API_KEY is not set
[OK] tool kilocode
[OK] tool opencode
diagnosis: 4 OK / 1 warning / 0 NG
```

## TUI Dashboard (optional)

`provsync-tui` is a terminal dashboard for selecting provider/tool pairs with checkboxes and applying them after confirmation. The core `go.mod` carries no external dependencies; the TUI is isolated as a separate module under `tui/` and is opt-in.

```bash
make install    # installs both provsync and provsync-tui
provsync-tui
```

To build manually, run `go build -o provsync-tui .` inside `tui/`.

- The TUI calls the `provsync` binary as a child process (`provsync status --json` to build the view, `provsync push <tool> --provider <p> --write` to apply). It never computes changes itself.
- Nothing is written until you approve on the confirmation screen. The confirmation screen shows a push preview (semantic diff, without `--write`) under each command, so you can see what will change before approving.
- On the done screen, `u` reverts the last operation (runs `undo` after its own confirmation).
- With a non-terminal stdin it errors out with an "interaction required" message. The message language follows `PROVSYNC_LANG` / `LANG` (English by default, Japanese for locales starting with `ja`).
- `PROVSYNC_BIN` overrides the provsync binary path (default: `provsync` from PATH).

## Secret Handling

- The central config holds only `apiKeyEnv` (an environment variable name). Actual keys are never mediated.
- On `pull`, secret-like fields (`apiKey` / `api_key` / `token` / `secret` / `password` / `accessToken` / `access_token`, including inside `options`) are dropped with a warning.
- On `push`, secret fields that already exist in the target tool config are preserved as-is (never deleted, never leaked into the central config).
- `apiKeyEnv` is never rendered for opencode (its keys live in `auth.json`).
- `diff` output masks values of secret-like keys as `********` by default. The written file content is never masked (display only).
- To inspect actual values, pass `--show-secrets`. Values are shown with a warning at the top.
- `status` / `list` show key-name warnings only; values are never printed.
- See [Security](security.en.md) for how to report a vulnerability.

## Re-serialization Caveat

Tool configs are fully re-serialized (2-space indent, alphabetically sorted keys). Original comments and formatting are lost, so diffs may include formatting churn.
