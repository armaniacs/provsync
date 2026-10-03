package i18n

// jaCatalog は ja(正)のメッセージカタログ。ID → テンプレート。
// テンプレートは末尾に改行を含まない。%w は含めない(ラップは呼び出し側)。
var jaCatalog = map[string]string{
	// ---- platform ----
	"err.unsupportedOS": "未対応の OS です: Windows (対応: macOS / Linux)",

	// ---- store ----
	"err.central.read":    "中央設定を読めません",
	"err.central.invalid": "中央設定の JSON が不正です",

	// ---- backup ----
	"err.manifest.invalid":    "バックアップマニフェストの JSON が不正です",
	"err.undo.none":           "undo できる操作がありません",
	"err.undo.notFound":       "操作 %q が見つかりません",
	"err.backup.read":         "バックアップを読めません (%s)",
	"err.backup.hashMismatch": "バックアップのハッシュが一致しません (%s)",

	// ---- syncer ----
	"err.providers.notFound": "provider が見つかりません: %v",
	"warn.alias.undefined":   "エイリアス %q はツール %q 向けのモデル ID が未定義のため素通しします",

	// ---- lock / fsutil ----
	"err.lock.busy":       "別の provsync が実行中です (%s)",
	"err.symlink.resolve": "シンボリックリンクのリンク先を解決できません (%s)",

	// ---- adapter ----
	"err.adapter.unknownTool": "未知のツール %q です(有効: %s)",
	"warn.secret.field":       "秘密情報らしいフィールド %q を検出しました。秘密は仲介しないため取り込みません",
	"err.config.invalid":      "設定の %s が不正です (%s)",
	"err.config.missing":      "設定ファイルがありません: %s",
	"err.config.read":         "設定を読めません",

	// ---- cli: 共通 ----
	"err.unknownCommand":       "未知のコマンド %q です(--help を参照)",
	"err.keepEnv.invalid":      "PROVSYNC_KEEP は 1 以上の整数で指定してください (値: %s)",
	"err.write.failed":         "書き込みに失敗しました (%s)",
	"err.backup.failed":        "バックアップに失敗しました",
	"err.recordHistory.failed": "履歴の記録に失敗しました",
	"msg.noChanges":            "変更はありません",
	"msg.previewOnly":          "(プレビューのみ; 適用するには --write)",
	"msg.backupID":             "バックアップ: %s",
	"msg.writtenFiles":         "この操作で既に書き込み済みのファイル: %s",
	"msg.undoHint":             "ヒント: provsync undo でこの操作をまとめて復元できます",
	"msg.written":              "書き込み: %s",

	// ---- cli: フラグ説明 ----
	"flag.root":        "パス解決の基準ディレクトリ(テスト用)",
	"flag.write":       "変更をファイルへ書き込む(既定はプレビュー)",
	"flag.noBackup":    "バックアップを記録しない(非推奨)",
	"flag.provider":    "対象 provider(カンマ区切り・繰り返し可)",
	"flag.from":        "sync の取り込み元ツール",
	"flag.to":          "sync の反映先ツール",
	"flag.list":        "undo の履歴を表示",
	"flag.prune":       "undo の履歴を掃除する",
	"flag.keep":        "残す履歴数(--prune 用)",
	"flag.json":        "JSON で出力する(list / status / diff)",
	"flag.exitCode":    "差分があるとき終了コード 3 で終了する(status)",
	"flag.strict":      "エイリアス未定義のモデル名があるときエラーにする(push)",
	"flag.version":     "バージョンを表示",
	"flag.showSecrets": "diff の出力で秘密の値をそのまま表示する(非推奨)",
	"flag.help":        "ヘルプを表示",

	// ---- cli: usage ----
	"usage.header":         "使い方: provsync <command> [flags]\n\nコマンド:",
	"usage.commonFlags":    "\n共通フラグ:\n  --write          変更を書き込む(既定はプレビュー)\n  --provider <p>   対象 provider を限定(カンマ区切り)\n  --no-backup      バックアップを記録しない\n  --root <dir>     パス解決の基準を差し替える(テスト用)",
	"usage.filesHeader":    "設定ファイル:",
	"usage.central":        "  中央設定: %s",
	"usage.centralMissing": "  中央設定: %s (未作成)",
	"usage.createdBy":      "    provsync init <tool> --write で作成します",
	"usage.toolMissing":    "  %-9s %s (未作成)",
	"usage.backupDir":      "  バックアップ: %s",

	// ---- cli: usage のコマンド一覧 ----
	"usage.summary.list":       "対応ツールと設定パスを表示",
	"usage.summary.status":     "ツールと中央設定の同期状態を表示",
	"usage.summary.init":       "初回セットアップ(中央設定を作る)",
	"usage.summary.pull":       "ツール設定を中央設定へ取り込む",
	"usage.summary.push":       "中央設定をツール設定へ反映する",
	"usage.summary.sync":       "a を取り込み b へ反映する(--from 省略可)",
	"usage.summary.diff":       "from を to に適用した場合の差分を表示",
	"usage.summary.undo":       "直前または指定操作を復元する(--list で履歴)",
	"usage.summary.doctor":     "環境を診断する(通信しない)",
	"usage.summary.check":      "各 provider の API 到達可否を確認する(明示実行のみ)",
	"usage.summary.completion": "シェル補完スクリプトを出力",
	"usage.summary.version":    "バージョンを表示",

	// ---- cli: 使い方エラー ----
	"err.usage.init":              "使い方: provsync init [tool]",
	"err.usage.pull":              "使い方: provsync pull <tool>",
	"err.usage.push":              "使い方: provsync push <tool>",
	"err.usage.sync":              "使い方: provsync sync --from <a> --to <b>(--from は省略可)",
	"err.usage.diff":              "使い方: provsync diff <from> <to>",
	"err.usage.completion":        "使い方: provsync completion <bash|zsh|fish>",
	"err.completion.unknownShell": "未知のシェル %q です(有効: bash / zsh / fish)",

	// ---- cli: init ----
	"err.init.alreadyInitialized": "すでに初期化されています: %s\n日常の更新には provsync pull <tool> を使ってください",
	"err.init.noToolConfig":       "対応ツールの設定ファイルが見つかりません。探した場所:\n%s\nツールを先に設定してから再実行してください",
	"msg.init.detected":           "検出したツール: %s",
	"msg.init.multiple":           "複数のツール設定が見つかりました:",
	"msg.init.specifyTool":        "provsync init <tool> でツールを指定してください",
	"msg.init.noProviders":        "取り込める provider がありません",
	"msg.init.secretHint":         "秘密は中央設定に保存されません。環境変数名を中央設定の apiKeyEnv に設定してください",
	"msg.init.next":               "次に: provsync init %s --write で中央設定を作成します",
	"msg.init.nextSteps":          "次の手順:\n  1. provsync status              同期状態を確認する\n  2. provsync push <他のツール>    他のツールへ反映する(まずプレビュー)\n  3. 問題があれば provsync undo で元に戻せます",

	// ---- cli: push / sync / diff 前処理 ----
	"err.push.noCentral":  "中央設定がありません。先に pull / sync を実行してください",
	"err.sync.noCentral":  "中央設定がありません。--from を指定するか先に pull してください",
	"err.readTool.failed": "ツール設定を読めません (%s)",
	"err.tool.missing":    "ツール設定がありません: %s",
	"warn.route.unused":   "エイリアス %q の経路で使える provider がありません(apiKeyEnv が未設定)。変更せず残します",
	"err.alias.strict":    "エイリアス未定義のモデル名があります (--strict): %d 件",

	// ---- cli: list / status ----
	"msg.central":            "中央設定: %s (%d providers)",
	"msg.centralMissing":     "中央設定: %s (未作成)",
	"msg.toolMissing":        "%-9s %s (未作成)",
	"msg.tool":               "%-9s %s%s (%d providers)",
	"msg.noDrift":         "  差分なし",
	"status.op.notInTool":    "ツールに無い",
	"status.op.notInCentral": "中央に無い",
	"status.op.drift":        "差分あり",

	// ---- cli: diff / undo ----
	"warn.showSecrets":   "--show-secrets により秘密の値をそのまま表示しています",
	"msg.noChange":        "  変更なし",
	"msg.pruned":         "削除: %d 件",
	"msg.noHistory":      "履歴はありません",
	"msg.list.noBackup":  " [バックアップなし]",
	"msg.list.undone":    " [undo 済み]",
	"warn.undo.noBackup": "直近の書き込みはバックアップなしで行われたため、この undo はそれより前の状態に戻します",
	"msg.restored":       "復元しました: %s (%s)",
	"msg.redo":           "やり直し: provsync undo %s",
	"msg.undo.restored":  "  復元: %s",
	"msg.undo.removed":   "  削除: %s",

	// ---- cli: 描画 ----
	"label.warning":   "警告",
	"label.added":     "追加",
	"label.updated":   "更新",
	"label.unchanged": "変更なし",
	"warn.loosePerm":  "権限が緩い (%04o): %s  chmod %s %s",

	// ---- cli: doctor ----
	"doctor.name.paths":            "パス解決",
	"doctor.name.central":          "中央設定",
	"doctor.name.apiKeyEnv":        "apiKeyEnv %s",
	"doctor.name.tool":             "ツール %s",
	"doctor.name.perms":            "権限",
	"doctor.status.ok":             "OK",
	"doctor.status.warn":           "警告",
	"doctor.status.ng":             "NG",
	"doctor.detail.centralMissing": "未作成です。provsync init <tool> --write を実行してください",
	"doctor.detail.envMissing":     "環境変数 %s が未設定です",
	"doctor.detail.toolMissing":    "%s が未作成です",
	"doctor.detail.toolWarnings":   "%s",
	"doctor.detail.loosePerm":      "権限が緩い (%04o): %s  chmod 600 %s",
	"doctor.detail.statePermLoose": "状態ディレクトリの権限が緩い: %s  chmod 700 %s",
	"doctor.summary":               "診断結果: OK %d 件 / 警告 %d 件 / NG %d 件",
	"err.doctor.problems":          "診断で問題が見つかりました (NG %d 件)",

	// ---- cli: check ----
	"err.check.noCentral":    "中央設定がありません。先に pull / init を実行してください",
	"check.skip.noAPIKeyEnv": "[スキップ] %s: apiKeyEnv が未設定",
	"check.skip.noEnvVar":    "[スキップ] %s: 環境変数 %s が未設定",
	"check.skip.noBaseURL":   "[スキップ] %s: baseURL が未設定",
	"check.badRequest":       "[応答異常] %s: リクエストを構築できません",
	"check.unreachable":      "[到達不可] %s",
	"check.ok":               "[OK] %s (%dms)",
	"check.authFailed":       "[認証失敗] %s",
	"check.badStatus":        "[応答異常] %s: HTTP %d",

	// ---- cli: 詳細ヘルプ ----
	"help.list": `list - 対応ツールと設定パスを表示

用途: 各ツールの設定ファイルと中央設定のパス、provider 数を表示する。

使い方: provsync list

例:
  provsync list
`,
	"help.status": `status - ツールと中央設定の同期状態を表示

用途: ツール設定と中央設定の provider 差分( drift )を表示する。

使い方: provsync status [tool...]

引数: tool を省略すると全対応ツールを表示する。

関連フラグ:
  --json    JSON で出力する

例:
  provsync status
  provsync status kilocode
`,
	"help.init": `init - 初回セットアップ(中央設定を作る)

用途: ツール設定から中央設定を作成する。初回専用で、既存の中央設定は上書きしない。

使い方: provsync init [tool]

引数: tool を省略すると、設定ファイルが存在するツールを検出する。
      候補が複数ある場合は一覧を表示するので指定する。

関連フラグ:
  --write   中央設定を作成する(既定はプレビュー)

例:
  provsync init kilocode
  provsync init kilocode --write
`,
	"help.pull": `pull - ツール設定を中央設定へ取り込む

用途: ツール設定の provider エントリを中央設定へマージする。

使い方: provsync pull <tool>

引数: tool は kilocode(別名 kilo) / opencode。

関連フラグ:
  --write          中央設定へ書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync pull kilocode
  provsync pull kilocode --write
`,
	"help.push": `push - 中央設定をツール設定へ反映する

用途: 中央設定の provider エントリをツール設定へマージする。

使い方: provsync push <tool>

引数: tool は kilocode(別名 kilo) / opencode。

関連フラグ:
  --write          ツール設定へ書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync push opencode
  provsync push opencode --write
`,
	"help.sync": `sync - 取り込みと反映を一度に行う

用途: --from のツール設定を中央設定へ取り込み、--to のツール設定へ反映する。

使い方: provsync sync --from <a> --to <b>

引数: --from を省略すると中央設定をそのまま使う。

関連フラグ:
  --from <tool>   取り込み元ツール(省略可)
  --to <tool>     反映先ツール(必須)
  --write         ファイルへ書き込む(既定はプレビュー)

例:
  provsync sync --from kilocode --to opencode --write
`,
	"help.diff": `diff - 適用した場合の差分を表示

用途: from を to に適用した場合の意味差分と統合 diff を表示する。

使い方: provsync diff <from> <to>

引数: from / to はツール名か central。

関連フラグ:
  --show-secrets   秘密の値をそのまま表示する(非推奨)

例:
  provsync diff kilocode opencode
`,
	"help.undo": `undo - 直前または指定操作を復元する

用途: 書き込み操作をバックアップから復元する。--write は不要で直接適用する。

使い方: provsync undo [id]

引数: id を省略すると直前の書き込みを復元する。undo 自体を undo できる(redo)。

関連フラグ:
  --list   履歴を表示する
  --prune / --keep <n>   履歴を掃除する

例:
  provsync undo
  provsync undo --list
  provsync undo 20261002T093045-8c2d
`,
	"help.doctor": `doctor - 環境を診断する

用途: 設定の存在・構文・apiKeyEnv の環境変数・権限を一覧で診断する。

使い方: provsync doctor

通信せず、ファイルも書かない。NG があるときは終了コード 1 で終わる。

例:
  provsync doctor
`,
	"help.check": `check - API の疎通確認

用途: 中央設定の各 provider について API への到達可否を確認する。

使い方: provsync check

通信するのはこのコマンドだけ。他のコマンドから呼ばない。
タイムアウトは 1 provider あたり 5 秒。秘密の値は出力しない。

関連フラグ:
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync check
`,
	"help.completion": `completion - シェル補完スクリプトを出力

用途: bash / zsh / fish 用の補完スクリプトを標準出力へ出す。

使い方: provsync completion <shell>

引数: shell は bash / zsh / fish。

例:
  # zsh
  provsync completion zsh > "${fpath[1]}/_provsync"

  # bash
  source <(provsync completion bash)
`,
	"help.version": `version - バージョンを表示

用途: バイナリのバージョンを 1 行で表示する。

使い方: provsync version

例:
  provsync version
  provsync --version
`,
}
