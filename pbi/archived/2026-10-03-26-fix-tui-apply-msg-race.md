# PBI: TUI の非同期適用を Msg 経由に修正し画面遷移を網羅テストする

## 種別
- fix（データ競合の解消）+ test（画面遷移の網羅テスト追加）

## ユーザーストーリー
provsync-tui の保守担当者として、`applySelected` が返す tea.Cmd クロージャ内でモデルのフィールドを直接書き換えているため、bubbletea の契約（モデルは Update 内でのみ変更する）に沿って Msg 経由に修正したい、なぜなら Cmd は別 goroutine で実行されるためイベントループ（Update/View）とのデータ競合になり、Go のメモリモデル上は未定義動作になるから

## 優先度
- 順位: 2
- RICEスコア: 3.33（Reach=5 / Impact=2 / Confidence=1.0 / Effort=3）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticb.md` の候補 C1。前ラウンド台帳の積み残し「tui Update 網羅テスト（confirm→done→quit）」を統合

## 背景（レビューで確認済みの実在箇所。行番号は概略）
- `tui/tui.go` の `applySelected`（概略 97-113 行）は `func() tea.Msg` クロージャを返し、クロージャ内の末尾で `m.message = out` と `m.screen = screenDone` を実行してから `tea.Quit()` を返している。bubbletea は Cmd を別 goroutine で実行するため、このモデル書き換えはイベントループと競合する
- `listModel` のフィールド `message []string` と `screen screen` は `Update` / `View` から読まれる（tui.go の Update/View）
- 現状の TUI テスト（tui/model_test.go、tui/i18n_test.go）は selection と i18n 文言のテストが中心で、confirm→done→quit の画面遷移を網羅するテストがない

## 改善案
1. `type applyDoneMsg struct { messages []string }` のような Msg 型を定義する
2. `applySelected` の Cmd クロージャは子プロセス実行結果のメッセージ列を組み立て、`applyDoneMsg` を返すだけにする（モデルを書き換えない）
3. `Update` に `case applyDoneMsg:` を追加し、`m.message` と `m.screen = screenDone` を設定して `tea.Quit` を返す
4. 遷移の網羅テストを追加する: list→confirm（enter）→done（y/enter + applyDoneMsg）→quit（q）の一連の Update 呼び出しを pin する。confirm→list（n/esc）の復帰も含める

## 制約
- 外部挙動（画面遷移・文言・子プロセス呼び出しの引数）を変えない
- tui モジュールは独立モジュール（tui/go.mod、replace で親を参照）。依存追加はしない
- 親モジュール（internal/cli など）には触れない

## BDD受け入れシナリオ
Scenario: 適用完了が競合なく画面へ反映される
  Given tui で drift があり confirm 画面まで進んでいる
  When  y を押して適用 Cmd が完了する
  Then  applyDoneMsg が Update で処理され、done 画面に遷移してメッセージが表示される

Scenario: 画面遷移が網羅テストで pin されている
  Given tui のモデルテストがある
  When  list→confirm→done→quit と confirm→list の遷移をテストする
  Then  各遷移がテストで検証され、回帰で壊れたら失敗する

## 受け入れ基準
1. `applySelected` の Cmd クロージャがモデルのフィールドを書き換えない（Msg を返すのみ）
2. `Update` が `applyDoneMsg` を処理して message/screen を設定する
3. confirm→done→quit と confirm→list の遷移テストが存在し、パスする
4. `make tui-check`（tui モジュールの build + test）がパスする
5. 既存の tui テストが変更なしでパスする
6. `go test -race`（tui モジュール）で競合検出がない
7. 外部挙動（表示文言・遷移・子プロセス引数）が変わらない

## テスト戦略
- 遷移テストは `tea.KeyMsg` と `applyDoneMsg` を Update に直接渡して検証する（bubbletea プログラム全体の起動は不要）
- 子プロセス実行部分は既存の fakeBin パターン（tui/i18n_test.go 参照）を流用できる
- `go test -race ./...`（tui モジュール内）を実行する

## 見積もり
3 SP

## Definition of Done
- [x] Cmd クロージャがモデルを書き換えない（Msg 経由）
- [x] Update が applyDoneMsg を処理する
- [x] 遷移網羅テスト（confirm→done→quit、confirm→list）がパスする
- [x] `make tui-check` がパスする
- [x] `go test -race`（tui モジュール）がパスする
- [x] 外部挙動が不変である

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`tui/tui.go` 全体、`tui/model.go`、`tui/model_test.go`、`tui/i18n_test.go`、`tui/go.mod`。

### 手順
1. 遷移テストを先に書く（confirm→done→quit、confirm→list）。applyDoneMsg をまだ処理しない現行実装では、done 遷移のテストはクロージャ実行が必要になるため、テストは Msg 経由を前提に書く（RED）
2. `applyDoneMsg` 型を定義し、`applySelected` を Msg 返却に修正する
3. `Update` に `case applyDoneMsg:` を追加する（GREEN）
4. `cd tui && go test ./...` と `go test -race ./...` を実行する
5. `make tui-check` を実行する

### 注意
- 担当ファイルは `tui/` 配下のみ。`internal/` 配下・ルートのファイルには触れない
- bubbletea のバージョン（tui/go.mod の v1.3.10）は変えない
- `make tui-check` が 3 回直しても通らない場合は、00-implementation-guide.md §7 に従い実装を止めて報告する
