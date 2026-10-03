# PBI INDEX

PBI の進行状況と台帳を管理する。形式の詳細は [00-implementation-guide.md](00-implementation-guide.md)。

## 進行中

### 2026-10-03 ラウンド b（大局的コード改善）

| 順位 | PBI | RICE |
|---|---|---|
| 1 | [2026-10-03-28](2026-10-03-28-docs-structure-map-drift.md) | 7.0 |
| 2 | [2026-10-03-26](2026-10-03-26-fix-tui-apply-msg-race.md) | 3.33 |
| 3 | [2026-10-03-29](2026-10-03-29-refactor-small-dedup.md) | 1.5 |
| 4 | [2026-10-03-27](2026-10-03-27-refactor-jsonc-single-pass.md) | 1.07 |

## 統合台帳

| 台帳 | トピック | 状態 |
|---|---|---|
| [2026-10-03-00-backlog.md](2026-10-03-00-backlog.md) | 2026-10-03 ラウンド（ユーザー要求 20 PBI） | 全 20 件アーカイブ済み |
| [2026-10-03-00-backlog-holistic.md](2026-10-03-00-backlog-holistic.md) | 2026-10-03 ラウンド（大局的コード改善・4 PBI） | 全 4 件アーカイブ済み |
| [2026-10-03-00-backlog-holisticb.md](2026-10-03-00-backlog-holisticb.md) | 2026-10-03 ラウンド b（大局的コード改善・4 PBI） | 4 件進行中 |

## アーカイブ履歴

| 日付 | アーカイブ | 内容 |
|---|---|---|
| 2026-10-03 | [2026-10-03 バッチ](archived/) | PBI 01〜19（バージョン表示、設定パス表示、SECURITY.md、秘密マスク、init、CI、CLI UX、バックアップ保持・権限、リリース自動化、シンボリックリンク、Windows 非対応、--json、エイリアス、ファズ・ゴールデン、排他ロック、doctor、貢献者ドキュメント、TUI、経路フォールバック） |
| 2026-10-03 | [2026-10-03 バッチ](archived/) | PBI 20〜23（cli.go 責務分割、パイプライン共通化、JSON drift 構造化、コマンドレジストリ統一） |
| 2026-10-03 | [2026-10-03-24](archived/2026-10-03-24-feat-i18n-all-messages.md) | CLI メッセージを含む全ユーザー向け出力の i18n 化（ja/en）。実装・テスト・規約更新完了。GitHub PR 承認は別途 |
| 2026-10-03 | [2026-10-03-25](archived/2026-10-03-25-backlog-tui-i18n.md) | tui ダッシュボードの i18n 化（ja/en、replace 指令でカタログ共用）。GitHub PR 承認は別途 |
