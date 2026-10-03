# PBI: symlink 解決の重複を fsutil の共有ヘルパーに集約する

## 種別
- refactor（挙動不変・最小差分）

## ユーザーストーリー
provsync の保守担当者として、Lstat→symlink 判定→EvalSymlinks の解決手順が `checkReadable` と `resolveWritePath` の 2 箇所にあるため、fsutil の共有ヘルパーに集約したい、なぜなら symlink 方針の変更時に両方の同期が必要になり片方だけ直った状態を作りやすいから

## 優先度
- 順位: 1
- RICEスコア: 1.6（Reach=2 / Impact=1 / Confidence=0.8 / Effort=1）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticc.md` の候補 C4

## 背景（レビューで確認済みの実在箇所。行番号は概略）
- `internal/cli/sync.go` の `checkReadable`（概略 362 行目付近）: Lstat し、欠落なら `err.tool.missing`、symlink なら EvalSymlinks しリンク切れなら `err.symlink.resolve` でラップして返す
- `internal/fsutil/fsutil.go` の `resolveWritePath`（概略 69 行目付近）: Lstat し、欠落ならパスをそのまま返し、symlink なら EvalSymlinks しリンク切れなら `err.symlink.resolve` でラップして返す
- 両者の欠落時動作は意図的に異なる（読み取り経路はエラー、書き込み経路はパススルー）ため、この差は維持しなければならない

## 改善案
1. `internal/fsutil` に symlink 解決の export ヘルパー（例: `ResolveSymlinkTarget(path string) (string, error)`。Lstat→symlink 判定→EvalSymlinks を行い、リンク切れは `err.symlink.resolve` でラップ、欠落は呼び出し元に判断させる形でそのまま返す）を作る
2. `checkReadable` は欠落判定（`err.tool.missing`）だけ残し、解決部分をヘルパーへ委譲する
3. `resolveWritePath` も同じヘルパーを使う形に寄せる（欠落時のパススルー動作は不変）
4. いずれも外部挙動・出力・終了コードは 1 バイトも変えない

## 制約
- 外部挙動を一切変えない（欠落時の差: 読み取りは `err.tool.missing`、書き込みはパススルーを維持）
- 公開 API の既存シグネチャを変えない。新設は fsutil の export ヘルパー 1 つのみ
- 既存テストの期待値を書き換えない

## BDD受け入れシナリオ
Scenario: 壊れたシンボリックリンクのエラーが変わらない
  Given リンク切れのツール設定がある
  When  push/diff 等で checkReadable を通る
  Then  `err.symlink.resolve` のメッセージが変更前と同一である

Scenario: symlink 経由の書き込みが維持される
  Given シンボリックリンクの中央設定がある
  When  書き込みを行う
  Then  リンク先の実体が置換されリンクが維持される（従来どおり）

## 受け入れ基準
1. `checkReadable` と `resolveWritePath` が同一の解決ヘルパーを通る
2. 全テストが変更なしでパスする
3. `make check` がパスする
4. `go vet` / staticcheck の新規 error が 0 件
5. symlink 関連の既存テスト（壊れたリンク・リンク経由書き込み）が変更なしでパスする

## テスト戦略
- 既存の symlink テスト（adapter/cli/fsutil の関連テスト）がそのまま通ることを確認する（parity）
- 新規テストは追加しない（挙動不変のため）。ヘルパー単体の正常・リンク切れの確認は既存テスト経由で間接的にカバーされる

## 見積もり
1 SP

## Definition of Done
- [x] fsutil に symlink 解決の共有ヘルパーがある
- [x] `checkReadable` がヘルパーへ委譲している（欠落判定は維持）
- [x] `resolveWritePath` がヘルパーを使っている（パススルー動作は維持）
- [x] 全テストが変更なしでパスする
- [x] `make check` がパスする

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`internal/fsutil/fsutil.go` 全体、`internal/cli/sync.go` の `checkReadable` 付近、symlink 関連の既存テスト。

### 手順
1. fsutil に export ヘルパーを作る（欠落は呼び出し元判断のため素通し、リンク切れは `err.symlink.resolve` でラップ）
2. `checkReadable` と `resolveWritePath` をヘルパー使用に書き換える
3. `go test ./internal/fsutil/ ./internal/cli/ ./internal/adapter/` が変更なしで通ることを確認する
4. `make check` を実行する

### 注意
- 担当ファイルは `internal/fsutil/fsutil.go`（+ 必要なら同パッケージのテスト）と `internal/cli/sync.go` のみ。`internal/backup/`、`tui/`、`internal/cli/status.go`・`doctor.go` には触れない
- 挙動不変のため共通ガイドの「テストを先に書く」は既存テストの parity 確認として適用する
- `make check` が 3 回直しても通らない場合は、00-implementation-guide.md §7 に従い実装を止めて報告する
