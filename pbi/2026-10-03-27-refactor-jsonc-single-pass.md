# PBI: jsonc の文字列走査を単一パスに統合する

## 種別
- refactor（挙動不変。出力を 1 バイトも変えず、内部構造だけを整える）

## ユーザーストーリー
provsync の保守担当者として、`StripJSONC` 内部の 2 つの走査関数が同一の文字列認識ループ（inString/escaped ステートマシン）を二重実装しているため、単一パスに統合して一貫性を担保したい、なぜならブロックコメント対応やエスケープ処理の修正の際に 2 箇所を一貫して修正する必要があり、片方だけの修正がバグの温床になるから

## 優先度
- 順位: 4
- RICEスコア: 1.07（Reach=4 / Impact=1 / Confidence=0.8 / Effort=3）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticb.md` の候補 C2。前ラウンド台帳の積み残し「jsonc / diff 個別精読」の成果（diff.go は構造的課題なし、jsonc に重複あり）

## 背景（レビューで確認済みの実在箇所。行番号は概略）
- `internal/jsonc/jsonc.go` の `stripLineComments`（概略 10-45 行）と `removeTrailingCommas`（概略 47-82 行）が、ともに `inString` / `escaped` の 2 変数で文字列リテラル内かを追跡する同一構造のバイト走査ループを実装している
- `StripJSONC` は `removeTrailingCommas(stripLineComments(b))` として 2 パスで処理する
- 「変わる理由が同じ」: ブロックコメント対応（現状は非対応と明記、jsonc.go 冒頭のコメント）、エスケープ処理の修正は両方のループに波及する
- 既存の検証手段: `internal/jsonc` にはユニットテストと `FuzzStripJSONC` ファズテスト（Makefile の `make fuzz` で実行）がある

## 改善案
単一パスの走査に統合する。1 つのループでコメント除去（文字列外の `//` から行末まで削除、改行は保持）と末尾カンマ除去（文字列外の `,` の後に空白のみ挟んで `}` か `]` が来るなら削除）を同時に行う。公開 API `StripJSONC(b []byte) []byte` のシグネチャと出力は 1 バイトも変えない。
実装者が「1 パス統合」ではなく「文字列走査部分の共通ヘルパー抽出」を選んでもよい。判定基準は「ブロックコメント対応を入れるときに 1 箇所の修正で済むか」である。どちらの場合も出力は byte-identical でなければならない。

## 制約
- `StripJSONC` の出力を 1 バイトも変えない（既存テストとファズで pin する）
- 公開 API・パッケージ構成を変えない（`internal/jsonc/jsonc.go` 内の整理のみ）
- 外部依存を追加しない

## BDD受け入れシナリオ
Scenario: 出力が 1 バイトも変わらない
  Given 既存の jsonc テストと FuzzStripJSONC が通る状態である
  When  stripLineComments と removeTrailingCommas を単一パスに統合する
  Then  全テストが変更なしでパスし、統合前後の出力が byte-identical である

Scenario: 文字列内のコメント風トークンが保持される
  Given `{"a": "http://x//y", "b": [1,2,]}` のような入力がある
  When  StripJSONC を実行する
  Then  文字列内の `//` と末尾カンマの扱いが統合前と完全に同一である

## 受け入れ基準
1. 既存の internal/jsonc テストが変更なしでパスする
2. 文字列リテラルの走査（inString/escaped 追跡）がコード上 1 箇所になっている
3. 統合前後の出力が byte-identical である（リファクタ前に現行出力を pin して確認する）
4. `make check` がパスする
5. `go vet` / staticcheck の新規 error が 0 件
6. ブロックコメント対応を入れる場合に修正箇所が 1 箇所で済む構造になっている

## テスト戦略
- リファクタ前に現行の `StripJSONC` 出力を pin する: 既存テストケースに加え、統合前の関数で代表的な入力（文字列内 `//`、エスケープされた `\"`、`[1,2,]`、`{"a":1,}`、コメント+末尾カンマの混在）の出力を記録する
- リファクタ後に pin した出力と diff が空であることを確認する
- `make fuzz`（FuzzStripJSONC、20 秒）を最後に実行する

## 見積もり
3 SP

## Definition of Done
- [ ] 文字列走査のステートマシンが 1 箇所になっている
- [ ] 既存テストが変更なしでパスする
- [ ] pin した出力との diff が空（byte-identical）
- [ ] `make check` がパスする
- [ ] `make fuzz` が失敗しない

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`internal/jsonc/jsonc.go` 全体、`internal/jsonc` のテストファイル、Makefile の fuzz ターゲット。

### 手順
1. pin テストを先に用意する: 現行 `StripJSONC` の出力を固定値として記録するテストを追加する（統合後も通る形にする。現行実装のスナップショットを出力し、期待値として埋め込む）
2. 単一パスの走査に統合する（または共通ヘルパーを抽出する）。公開 API と出力は不変
3. `go test ./internal/jsonc/` がパスすることを確認する
4. pin した出力との diff が空であることを確認する
5. `make check` を実行する
6. `make fuzz` を実行する

### 注意
- 担当ファイルは `internal/jsonc/` 配下のみ。他のパッケージ（adapter からの呼び出し側含む）には触れない
- 本 PBI は挙動不変の refactor のため、共通ガイドの「テストを先に書く」は pin テスト（現行出力の固定）として適用する。期待値の変更はしない
- `make check` が 3 回直しても通らない場合は、00-implementation-guide.md §7 に従い実装を止めて報告する
