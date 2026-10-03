# PBI: backup 記録経路の小規模不備を硬化する

## 種別
- fix（失敗経路の硬化）+ refactor（共通化）。いずれも外部の正常系挙動は変えない

## ユーザーストーリー
provsync の保守担当者として、`internal/backup/backup.go` の記録経路に 3 つの小規模不備（prune 二重実装・ID 衝突可能性・失敗時の orphan 残存）があるため、同一ファイル内でまとめて硬化したい、なぜなら記録経路は undo の信頼性の土台であり、小さな不備でもバックアップの欠損や無音の破損につながるから

## 優先度
- 順位: 2
- RICEスコア: 1.5（Reach=3 / Impact=1 / Confidence=1.0 / Effort=2）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticc.md` の候補 C3

## 背景（レビューで確認済みの実在箇所。行番号は概略）
すべて `internal/backup/backup.go` 内:
- `prune`（概略 227 行目付近）と `Prune`（概略 243 行目付近）が「N 件を超えた履歴と実体ディレクトリを削除」の同一ロジックを二重実装している（前ラウンドの `appendAndSave` 抽出時に残った残渣）
- `newID`（概略 314 行目付近）がタイムスタンプ + 2 ランダムバイトで ID を生成する。同秒・同値の衝突時は同一 opDir へ `capture` が上書きし、index に同 ID が2件載る（`Find` は先勝のため誤った内容を復元しうる）
- `Record`（概略 100 行目付近）が `capture` 失敗時に作成済み opDir を残したままエラーを返し、未索引の orphan ディレクトリが残る（prune の対象外のため蓄積する）

## 改善案
1. `removeBeyond(idx *index, keep int) error` のような非公開ヘルパーに「N 件超過分の履歴削除 + 実体ディレクトリ削除」を集約し、`prune` と `Prune` の両方から呼ぶ（`Prune` の load/save と戻り値の件数は不変）
2. `newID` のランダム部を 8 バイト（16 hex 文字）に拡張する。ID 形式 `timestamp-hex` は不変で、`Find` は完全一致のため既存操作との互換性は保たれる
3. `Record` が `capture` 失敗時に作成済み opDir を best-effort で除去してからエラーを返す（除去失敗は無視して元のエラーを返す）
4. 正常系の外部挙動・出力・終了コードは変えない

## 制約
- 正常系の外部挙動を変えない（ID は不透明文字列として扱い、形式依存のコードを増やさない）
- 公開 API（`Store` のメソッド群）のシグネチャを変えない
- 既存テストの期待値を書き換えない

## BDD受け入れシナリオ
Scenario: 連続記録で ID が衝突しない
  Given 空の Store がある
  When  短時間に 1000 件の ID を生成する
  Then  すべて一意である（現行 2 バイトでは birthday bound で衝突しうる）

Scenario: 記録失敗時に orphan が残らない
  Given 読み取り不可のファイルを含む paths がある
  When  `Record` が失敗する
  Then  エラーが返り、backups 配下に未索引のディレクトリが残らず、index の件数が変わらない

Scenario: 保持ポリシー変更が 1 箇所で済む
  Given `removeBeyond` に集約されている
  When  `Record` / `RecordMarker` / `Restore` / `Prune` が履歴を整理する
  Then  全経路が同じヘルパーを通る

## 受け入れ基準
1. `prune` と `Prune` が `removeBeyond` 系の単一ヘルパーを通る
2. `newID` のランダム部が 8 バイトになっている
3. `Record` の `capture` 失敗時に opDir が除去される
4. 上記 3 点の新規テスト（ID 一意性・orphan 不在・保持共通化の回帰）が存在しパスする
5. 既存の backup テストが変更なしでパスする
6. `make check` がパスする
7. `go vet` / staticcheck の新規 error が 0 件

## テスト戦略
- 失敗注入テストを先に書く（RED: 現行実装では ID 一意性テストが失敗しうること、orphan が残ることを確認してから GREEN 化する。ID 一意性は確率的なため、1000 件連番で現行 2 バイトの衝突期待値が高いことを利用する）
- 既存の保持テスト（`TestRetention` 等）が変更なしでパスすることを確認する

## 見積もり
2 SP

## Definition of Done
- [ ] `prune`/`Prune` が単一ヘルパーを通る
- [ ] `newID` のランダム部が 8 バイトになっている
- [ ] `Record` 失敗時に orphan が残らない
- [ ] 新規テスト（ID 一意性・orphan 不在）がパスする
- [ ] 既存テストが変更なしでパスする
- [ ] `make check` がパスする

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`internal/backup/backup.go` 全体、`internal/backup/backup_test.go`（特に `TestRetention` と `TestRestoreDetectsTamperedBackup` の注入パターン）。

### 手順
1. 失敗注入テストを先に書く（ID 一意性 1000 件、読み取り不可ファイルでの Record 失敗時の orphan 不在）。現行実装での失敗を確認する
2. `removeBeyond` を抽出し `prune`/`Prune` から呼ぶ
3. `newID` を 8 バイト化する
4. `Record` の失敗時に opDir を best-effort 除去する
5. `go test ./internal/backup/` がパスすることを確認する
6. `make check` を実行する

### 注意
- 担当ファイルは `internal/backup/` 配下のみ。`internal/fsutil/`、`internal/cli/`、`tui/` には触れない
- ID 形式の変更が `Find` や undo 経路に影響しないことを既存テストで確認する（ID は不透明文字列として扱う）
- `make check` が 3 回直しても通らない場合は、00-implementation-guide.md §7 に従い実装を止めて報告する
