# PBI: internal/cli と internal/backup の小規模重複を解消する

## 種別
- refactor（挙動不変・最小差分。出力・終了コード・テスト結果を一切変えない）

## ユーザーストーリー
provsync の保守担当者として、同じ理由で同時に変わるべき小さな処理が複数箇所に散在しているため、ヘルパーに集約して修正漏れの温床を潰したい、なぜなら散在した重複は 1 箇所の修正が他に飛び火せず、片方だけ直った状態を作りやすいから

## 優先度
- 順位: 3
- RICEスコア: 1.5（Reach=3 / Impact=1 / Confidence=1.0 / Effort=2）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticb.md` の候補 C4

## 背景（レビューで確認済みの実在箇所。行番号は概略）
4 つの小規模重複がある（すべて実在確認済み）:

1. `internal/backup/backup.go` の `Record`（概略 107-117 行）、`RecordMarker`（概略 130-140 行）、`Restore`（概略 203-209 行）が `idx.Operations = append(...)` → `s.prune(idx)` → `s.save(idx)` の 3 行パターンを 3 箇所で繰り返している
2. バージョン 1 行出力が 3 箇所: `internal/cli/cli.go` の `RunWith`（概略 97 行、`fmt.Fprintf(out, "provsync %s\n", version.String())`）、`internal/cli/help.go` の `printUsage`（概略 80 行）、`internal/cli/help.go` の `cmdVersion`（概略 111 行）
3. `internal/cli/check.go` の `cmdCheck`（概略 34-49 行）が provider のキー一覧を構築した後、`--provider` 指定時には `FilterProviders` の結果からキー一覧を再構築しており、最初の構築が無駄になる二重構造（`FilterProviders` は keys が空なら全件のコピーを返すため、filter を先に行えば構築は 1 回で済む）
4. 権限緩判定 `Perm()&0o077 != 0` が 2 箇所: `internal/cli/doctor.go` の `doctorCheckPerms`（概略 171 行）と `filePermLoose`（概略 180-183 行）、および `internal/cli/render.go` の `warnLoosePerm`（概略 26-40 行。`filePermLoose` は doctor.go 側にのみ存在し render.go からは使えない）

## 改善案
1. `backup.go`: `appendAndSave(idx *index, op Operation) error` のような非公開ヘルパーを抽出し、3 箇所から呼ぶ
2. バージョン出力: `func printVersion(out io.Writer)` のような小さなヘルパーを internal/cli 内に作り、3 箇所から呼ぶ（フォーマット `"provsync %s\n"` は不変）
3. `check.go`: `FilterProviders` を先に呼び、キー一覧の構築を 1 回にする（`o.keys()` が空でも `FilterProviders` は全件のコピーを返すため出力は不変）
4. 権限緩判定: `filePermLoose` を internal/cli 内の共有ヘルパーとして render.go からも使う（判定 `Perm()&0o077 != 0` は不変）。render.go の `warnLoosePerm` は現状 `os.Stat` のモードをそのまま使っているため、必要なら「緩いか判定」と「モード取得」の分離も検討する

実装者は 4 件のうち実害・効果の小さいものを省略してよい。省略する場合は PBI の DoD から該当項目を除いて理由を記録する。

## 制約
- 外部挙動を一切変えない（出力バイト列・終了コード・テスト結果は全く同じ）
- 公開 API（大文字開始の関数）のシグネチャを変えない。新設するのは非公開ヘルパーのみ
- `internal/cli/cli_test.go` 等の既存テストを書き換えない

## BDD受け入れシナリオ
Scenario: 重複解消後も出力が 1 バイトも変わらない
  Given 既存の全テストが通る状態である
  When  4 箇所の小規模重複をヘルパーに集約する
  Then  全テストが変更なしでパスし、`provsync version` / `--help` / `check` / `status` の出力が移動前と 1 バイトも変わらない

Scenario: バックアップ記録のパターンが 1 箇所になる
  Given backup.go に append+prune+save のヘルパーがある
  When  Record / RecordMarker / Restore が履歴を追加する
  Then  3 経路が同じヘルパーを通り、将来の保持ポリシー変更が 1 箇所の修正で済む

## 受け入れ基準
1. 全テストが変更なしでパスする
2. `make check` がパスする
3. `go vet` / staticcheck の新規 error が 0 件
4. backup.go の append+prune+save パターンが 1 箇所のヘルパーになっている（3 経路から呼ばれる）
5. バージョン出力が internal/cli 内の 1 箇所のヘルパーに集約されている
6. 出力が byte-identical（`provsync version` / `--help` / `status` / `doctor` の出力が変更前と同じ）
7. 省略した重複がある場合は、その理由が PBI に記録されている

## テスト戦略
- 変更前に現行出力を pin する（`make build` 後に `bin/provsync version` / `--help` / `status` / `doctor` の出力を保存）
- 変更後に pin した出力との diff が空であることを確認する
- 既存の backup テスト（internal/backup/backup_test.go）と cli テストが変更なしでパスすることを確認する

## 見積もり
2 SP

## Definition of Done
- [ ] 全テストが変更なしでパスする
- [ ] `make check` がパスする
- [ ] backup.go の append+prune+save が 1 箇所のヘルパーになっている
- [ ] バージョン出力が 1 箇所のヘルパーに集約されている
- [ ] 出力が byte-identical である
- [ ] 省略した重複の理由が記録されている（省略した場合）

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`internal/backup/backup.go`、`internal/cli/cli.go`、`internal/cli/help.go`、`internal/cli/check.go`、`internal/cli/render.go`、`internal/cli/doctor.go`、`internal/cli/cli_test.go`。

### 手順
1. 変更前に現行出力を pin する: `make build` 後、`bin/provsync version` / `--help` / `status --json` / `doctor` の出力を保存する
2. backup.go に非公開ヘルパーを抽出し、3 経路から呼ぶ
3. internal/cli にバージョン出力のヘルパーを作り、3 箇所から呼ぶ
4. check.go のキー構築を 1 回に整理する
5. `filePermLoose` を共有化する
6. `go test ./...` が変更なしで通ることを確認する
7. pin した出力との diff が空であることを確認する
8. `make check` を実行する

### 注意
- 本 PBI は挙動不変の refactor のため、共通ガイドの「テストを先に書く」は pin テスト（現行出力の固定）として適用する。期待値の変更はしない
- 担当ファイルは `internal/backup/backup.go` と `internal/cli/{cli,help,check,render,doctor}.go`（+ 必要なら同パッケージ内の新規小ファイル）のみ。`tui/`、`internal/jsonc/`、docs には触れない
- 移動・集約以外の変更（命名変更・再フォーマット・コメント整理）をしない
- `make check` が 3 回直しても通らない場合は、00-implementation-guide.md §7 に従い実装を止めて報告する
