package i18n

// enCatalog は en のメッセージカタログ。ja の mirror で、キー集合は
// jaCatalog と完全一致しなければならない(完全性テストが強制する)。
var enCatalog = map[string]string{
	// ---- platform ----
	"err.unsupportedOS": "unsupported OS: Windows (supported: macOS / Linux)",

	// ---- store ----
	"err.central.read":    "cannot read the central config",
	"err.central.invalid": "invalid JSON in the central config",

	// ---- backup ----
	"err.manifest.invalid":    "invalid JSON in the backup manifest",
	"err.undo.none":           "no operation to undo",
	"err.undo.notFound":       "operation %q not found",
	"err.backup.read":         "cannot read the backup (%s)",
	"err.backup.hashMismatch": "backup hash mismatch (%s)",

	// ---- syncer ----
	"err.providers.notFound": "providers not found: %v",
	"warn.alias.undefined":   "alias %q has no model ID for tool %q; passing the model name through unchanged",

	// ---- lock / fsutil ----
	"err.lock.busy":       "another provsync is running (%s)",
	"err.symlink.resolve": "cannot resolve the symlink target (%s)",

	// ---- adapter ----
	"err.adapter.unknownTool": "unknown tool %q (valid: %s)",
	"warn.secret.field":       "detected a secret-like field %q; secrets are never mediated, so it was not imported",
	"err.config.invalid":      "invalid %s in the config (%s)",
	"err.config.missing":      "config file not found: %s",
	"err.config.read":         "cannot read the config",

	// ---- cli: 共通 ----
	"err.unknownCommand":       "unknown command %q (see --help)",
	"err.keepEnv.invalid":      "PROVSYNC_KEEP must be an integer >= 1 (value: %s)",
	"err.write.failed":         "failed to write (%s)",
	"err.backup.failed":        "backup failed",
	"err.recordHistory.failed": "failed to record history",
	"msg.noChanges":            "no changes",
	"msg.previewOnly":          "(preview only; apply with --write)",
	"msg.backupID":             "backup: %s",
	"msg.writtenFiles":         "files already written by this operation: %s",
	"msg.undoHint":             "hint: provsync undo restores all files of this operation at once",
	"msg.written":              "wrote: %s",

	// ---- cli: フラグ説明 ----
	"flag.root":        "base directory for path resolution (for tests)",
	"flag.write":       "write changes to files (preview by default)",
	"flag.noBackup":    "do not record a backup (deprecated)",
	"flag.provider":    "providers to target (comma-separated, repeatable)",
	"flag.from":        "source tool for sync",
	"flag.to":          "destination tool for sync",
	"flag.list":        "show undo history",
	"flag.prune":       "clean up undo history",
	"flag.keep":        "history entries to keep (for --prune)",
	"flag.json":        "output as JSON (list / status / diff)",
	"flag.exitCode":    "exit with code 3 when drift exists (status)",
	"flag.strict":      "fail when model names have no alias mapping (push)",
	"flag.yes":         "assume yes to confirmations (scripts and TUI)",
	"flag.version":     "show version",
	"flag.showSecrets": "show secret values in diff output (deprecated)",
	"flag.help":        "show help",

	// ---- cli: usage ----
	"usage.header":         "usage: provsync <command> [flags]\n\ncommands:",
	"usage.commonFlags":    "\ncommon flags:\n  --write          write changes (preview by default)\n  --provider <p>   limit target providers (comma-separated)\n  --no-backup      do not record a backup\n  --root <dir>     override the base directory for path resolution (for tests)",
	"usage.filesHeader":    "config files:",
	"usage.central":        "  central config: %s",
	"usage.centralMissing": "  central config: %s (not created)",
	"usage.createdBy":      "    create it with provsync init <tool> --write",
	"usage.toolMissing":    "  %-9s %s (not created)",
	"usage.backupDir":      "  backups: %s",

	// ---- cli: usage のコマンド一覧 ----
	"usage.summary.list":       "show supported tools and config paths",
	"usage.summary.status":     "show the sync state of tools and the central config",
	"usage.summary.init":       "first-time setup (create the central config)",
	"usage.summary.pull":       "import a tool config into the central config",
	"usage.summary.push":       "apply the central config to a tool config",
	"usage.summary.sync":       "import from a and apply to b (--from optional)",
	"usage.summary.diff":       "show the diff of applying from to to",
	"usage.summary.undo":       "restore the last or a given operation (--list for history)",
	"usage.summary.cleanup":    "remove provsync-managed files (central config and state)",
	"usage.summary.doctor":     "diagnose the environment (no network)",
	"usage.summary.check":      "check API reachability of each provider (explicit runs only)",
	"usage.summary.completion": "print a shell completion script",
	"usage.summary.version":    "show version",

	// ---- cli: 使い方エラー ----
	"err.usage.init":              "usage: provsync init [tool]",
	"err.usage.pull":              "usage: provsync pull <tool>",
	"err.usage.push":              "usage: provsync push <tool>",
	"err.usage.sync":              "usage: provsync sync --from <a> --to <b> (--from optional)",
	"err.usage.diff":              "usage: provsync diff <from> <to>",
	"err.usage.cleanup":           "usage: provsync cleanup [--write] [--yes]",
	"err.usage.completion":        "usage: provsync completion <bash|zsh|fish>",
	"err.completion.unknownShell": "unknown shell %q (valid: bash / zsh / fish)",

	// ---- cli: init ----
	"err.init.alreadyInitialized": "already initialized: %s\nfor everyday updates, use provsync pull <tool>",
	"err.init.noToolConfig":       "no supported tool config was found. Looked in:\n%s\nCreate one of them first, then run again. Minimal example (%s):\n{\"provider\": {\"my-llm\": {\"name\": \"My LLM\", \"npm\": \"@ai-sdk/openai-compatible\", \"baseURL\": \"https://api.example.com/v1\", \"apiKeyEnv\": \"MY_LLM_API_KEY\"}}}",
	"msg.init.detected":           "detected tool: %s",
	"msg.init.multiple":           "multiple tool configs were found:",
	"msg.init.specifyTool":        "specify a tool with provsync init <tool>",
	"msg.init.noProviders":        "no providers to import",
	"msg.init.secretHint":         "secrets are not stored in the central config. Set the env var name in apiKeyEnv",
	"msg.init.next":               "next: create the central config with provsync init %s --write",
	"msg.init.nextSteps":          "next steps:\n  1. provsync status              check the sync state\n  2. provsync push <other-tool>   reflect into other tools (preview first)\n  3. provsync undo restores the previous state if anything goes wrong",

	// ---- cli: push / sync / diff 前処理 ----
	"err.push.noCentral":  "no central config. Run pull / sync first",
	"err.sync.noCentral":  "no central config. Pass --from or run pull first",
	"err.readTool.failed": "cannot read the tool config (%s)",
	"err.tool.missing":    "tool config not found: %s",
	"warn.route.unused":   "no usable provider on the %q route (apiKeyEnv is not set); leaving providers unchanged",
	"err.alias.strict":    "model names without alias mapping (--strict): %d",

	// ---- cli: list / status ----
	"msg.central":            "central config: %s (%d providers)",
	"msg.centralMissing":     "central config: %s (not created)\nrun provsync init <tool> --write to create it",
	"msg.toolMissing":        "%-9s %s (not created)",
	"msg.tool":               "%-9s %s%s (%d providers)",
	"msg.noDrift":            "  no drift",
	"status.op.notInTool":    "not in tool",
	"status.op.notInCentral": "not in central",
	"status.op.drift":        "drift",

	// ---- cli: diff / undo ----
	"warn.showSecrets":   "--show-secrets shows secret values as-is",
	"msg.noChange":       "  no change",
	"msg.pruned":         "removed: %d entries",
	"msg.noHistory":      "no history",
	"msg.list.noBackup":  " [no backup]",
	"msg.list.undone":    " [undone]",
	"warn.undo.noBackup": "the most recent write was made without a backup, so this undo restores an earlier state",
	"msg.restored":       "restored: %s (%s)",
	"msg.redo":           "redo: provsync undo %s",
	"msg.undo.restored":  "  restored: %s",
	"msg.undo.removed":   "  removed: %s",

	// ---- cli: cleanup ----
	"err.cleanup.interactive":    "interaction required (stdin is not a terminal)",
	"msg.cleanup.preview":        "the following will be removed:",
	"msg.cleanup.target":         "%s (%s)",
	"msg.cleanup.total":          "total: %s",
	"msg.cleanup.confirm":        "really delete? y / n",
	"msg.cleanup.cancelled":      "cancelled",
	"msg.cleanup.none":           "nothing to remove",
	"msg.cleanup.trashed":        "moved to Trash: %s",
	"msg.cleanup.deleted":        "deleted: %s",
	"warn.cleanup.trashFallback": "could not move to Trash, deleting directly: %s",
	"warn.cleanup.failed":        "failed to remove: %s",

	// ---- cli: 描画 ----
	"label.warning":   "warning",
	"label.added":     "added",
	"label.updated":   "updated",
	"label.unchanged": "unchanged",
	"warn.loosePerm":  "loose permissions (%04o): %s  chmod %s %s",

	// ---- cli: doctor ----
	"doctor.name.paths":            "path resolution",
	"doctor.name.central":          "central config",
	"doctor.name.apiKeyEnv":        "apiKeyEnv %s",
	"doctor.name.tool":             "tool %s",
	"doctor.name.perms":            "permissions",
	"doctor.status.ok":             "OK",
	"doctor.status.warn":           "warning",
	"doctor.status.ng":             "NG",
	"doctor.detail.centralMissing": "not created. Run provsync init <tool> --write",
	"doctor.detail.envMissing":     "env var %s is not set",
	"doctor.detail.toolMissing":    "%s is not created",
	"doctor.detail.toolWarnings":   "%s",
	"doctor.detail.loosePerm":      "loose permissions (%04o): %s  chmod 600 %s",
	"doctor.detail.statePermLoose": "state dir permissions are loose: %s  chmod 700 %s",
	"doctor.summary":               "diagnosis: %d OK / %d warning / %d NG",
	"err.doctor.problems":          "diagnosis found problems (%d NG)",

	// ---- cli: check ----
	"err.check.noCentral":    "no central config. Run pull / init first",
	"check.skip.noAPIKeyEnv": "[skip] %s: apiKeyEnv is not set",
	"check.skip.noEnvVar":    "[skip] %s: env var %s is not set",
	"check.skip.noBaseURL":   "[skip] %s: baseURL is not set",
	"check.badRequest":       "[bad response] %s: failed to build the request",
	"check.unreachable":      "[unreachable] %s",
	"check.ok":               "[OK] %s (%dms)",
	"check.authFailed":       "[auth failed] %s",
	"check.badStatus":        "[bad response] %s: HTTP %d",

	// ---- tui ----
	"err.tui.interactive":        "interaction required (stdin is not a terminal)",
	"err.tui.fetch":              "cannot run provsync status --json (%s)",
	"err.tui.invalidOutput":      "invalid output from provsync status --json",
	"err.tui.schema":             "unsupported schemaVersion: %d",
	"msg.tui.central":            "central config: %s",
	"msg.tui.confirmHeader":      "the following commands will run (with --write):",
	"msg.tui.confirmPrompt":      "run them? y / n",
	"msg.tui.previewLoading":     "loading change previews...",
	"msg.tui.confirmUndo":        "revert the last operation? y / n",
	"msg.tui.menuTitle":          "commands",
	"msg.tui.menuBack":           "press q for the menu",
	"msg.tui.footerList":         "↑↓/jk move · space select · enter confirm · m menu · q quit · ? help",
	"msg.tui.footerMenu":         "↑↓ select · enter run · esc back · q quit · ? help",
	"msg.tui.footerPrompt":       "enter submit · esc cancel · ? help",
	"msg.tui.footerPreview":      "y/enter run · n/esc back · ? help",
	"msg.tui.footerPreviewError": "preview failed · esc/q back to menu · ? help",
	"msg.tui.footerResult":       "q/enter menu · ? help",
	"msg.tui.footerConfirm":      "y/enter run · n/esc back · ? help",
	"msg.tui.footerDone":         "u undo · q quit · ? help",
	"msg.tui.helpTitle":          "shortcuts",
	"msg.tui.pickCentral":        "(central config)",
	"msg.tui.pickEmpty":          "(no tools found)",
	"msg.tui.footerPick":         "↑↓ select · enter pick · esc back · ? help",
	"msg.tui.resultCreateHint":   "central config is missing — press i to create it now",
	"msg.tui.helpBody": `list: ↑↓/jk move · space select · enter confirm · m menu · g/G top/bottom
menu: ↑↓ select · enter run · esc back · g/G top/bottom
pick: ↑↓ select · enter pick · esc back
prompt: type + enter submit · esc cancel
preview: y/enter run · n/esc back (blocked when the preview failed)
result: q/enter menu
confirm: y/enter run · n/esc back
done: u undo the last operation
global: q quit · ctrl+c quit · ? this help · esc close`,
	"msg.tui.promptTool":        "tool: ",
	"msg.tui.promptFrom":        "source (empty = central config): ",
	"msg.tui.promptTo":          "target: ",
	"msg.tui.promptId":          "operation id (empty = last): ",
	"msg.tui.promptProvider":    "provider (empty = all): ",
	"msg.tui.promptShell":       "shell (bash/zsh/fish): ",
	"msg.tui.menuPreviewHeader": "the following command will run:",
	"msg.tui.menuRunPrompt":     "run it? y / n",
	"msg.tui.apply":             "applied: provsync %s\n%s",
	"msg.tui.failed":            "failed: provsync %s: %s",

	// ---- cli: 詳細ヘルプ ----
	"help.list": `list - show supported tools and config paths

Purpose: show the config file paths of each tool and the central config, and the number of providers.

Usage: provsync list

Examples:
  provsync list
`,
	"help.status": `status - show the sync state of tools and the central config

Purpose: show the provider drift between tool configs and the central config.

Usage: provsync status [tool...]

Arguments: omit tool to show all supported tools.

Related flags:
  --json    output as JSON

Examples:
  provsync status
  provsync status kilocode
`,
	"help.init": `init - first-time setup (create the central config)

Purpose: create the central config from a tool config. First time only; an existing central config is never overwritten.

Usage: provsync init [tool]

Arguments: omit tool to detect tools that have a config file.
      When there are multiple candidates, a list is shown; specify one.

Related flags:
  --write   create the central config (preview by default)

Examples:
  provsync init kilocode
  provsync init kilocode --write
`,
	"help.pull": `pull - import a tool config into the central config

Purpose: merge provider entries from a tool config into the central config.

Usage: provsync pull <tool>

Arguments: tool is kilocode (alias kilo) / opencode.

Related flags:
  --write          write to the central config (preview by default)
  --provider <p>   limit target providers (comma-separated)

Examples:
  provsync pull kilocode
  provsync pull kilocode --write
`,
	"help.push": `push - apply the central config to a tool config

Purpose: merge provider entries from the central config into a tool config.

Usage: provsync push <tool>

Arguments: tool is kilocode (alias kilo) / opencode.

Related flags:
  --write          write to the tool config (preview by default)
  --provider <p>   limit target providers (comma-separated)

Examples:
  provsync push opencode
  provsync push opencode --write
`,
	"help.sync": `sync - import and apply in one step

Purpose: import the --from tool config into the central config, then apply it to the --to tool config.

Usage: provsync sync --from <a> --to <b>

Arguments: when --from is omitted, the central config is used as-is.

Related flags:
  --from <tool>   source tool (optional)
  --to <tool>     destination tool (required)
  --write         write to files (preview by default)

Examples:
  provsync sync --from kilocode --to opencode --write
`,
	"help.diff": `diff - show the diff of an apply

Purpose: show the semantic diff and unified diff of applying from to to.

Usage: provsync diff <from> <to>

Arguments: from / to are tool names or central.

Related flags:
  --show-secrets   show secret values as-is (deprecated)

Examples:
  provsync diff kilocode opencode
`,
	"help.undo": `undo - restore the last or a given operation

Purpose: restore write operations from backups. --write is not needed; undo applies directly.

Usage: provsync undo [id]

Arguments: omit id to restore the most recent write. An undo itself can be undone (redo).

Related flags:
  --list   show history
  --prune / --keep <n>   clean up history

Examples:
  provsync undo
  provsync undo --list
  provsync undo 20261002T093045-8c2d
`,
	"help.cleanup": `cleanup - remove provsync-managed files

Usage:
  provsync cleanup [--write] [--yes]

Removes the central config (~/.config/provsync/config.json) and the state
directory (~/.local/state/provsync: history, backups, lock). Tool configs
and binaries are never touched.

Without flags, prints what would be removed. --write executes after an
interactive confirmation. --yes executes without confirmation (for scripts
and the TUI). On macOS, files are moved to the Trash; elsewhere they are
deleted directly.`,
	"help.doctor": `doctor - diagnose the environment

Purpose: diagnose config existence, syntax, apiKeyEnv env vars, and permissions in a list.

Usage: provsync doctor

No network access and no file writes. Exits with code 1 when any NG exists.

Examples:
  provsync doctor
`,
	"help.check": `check - API reachability

Purpose: check API reachability for each provider in the central config.

Usage: provsync check

This is the only command that touches the network; it is never called from other commands.
The timeout is 5 seconds per provider. Secret values are never printed.

Related flags:
  --provider <p>   limit target providers (comma-separated)

Examples:
  provsync check
`,
	"help.completion": `completion - print a shell completion script

Purpose: print a completion script for bash / zsh / fish to stdout.

Usage: provsync completion <shell>

Arguments: shell is bash / zsh / fish.

Examples:
  # zsh
  provsync completion zsh > "${fpath[1]}/_provsync"

  # bash
  source <(provsync completion bash)
`,
	"help.version": `version - show version

Purpose: print the binary version in one line.

Usage: provsync version

Examples:
  provsync version
  provsync --version
`,
}
