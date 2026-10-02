# PBI: CI と品質ゲート（race / vet / staticcheck / govulncheck）を整備する

## ユーザーストーリー
メンテナとして、push と PR のたびに自動で品質検査を走らせたい、なぜなら `make check` を手元で忘れても壊れた変更を main に入れないため

## 優先度
- 順位: 06 / 19
- RICEスコア: 10（Reach=5 / Impact=1 / Confidence=100% / Effort=0.5）
- 根拠: 現状 CI なし。機能 PBI の回帰を守る基盤。以降の PBI 実装前に入れる価値が高い

## BDD受け入れシナリオ
Scenario: 正常な PR は通る
  Given 整形済みで vet・テストが通る変更がある
  When  PR を作成する
  Then  CI の全ジョブが成功する

Scenario: 未整形コードは落ちる
  Given gofmt されていないファイルを含む変更がある
  When  PR を作成する
  Then  fmt-check ジョブが失敗し、対象ファイルが表示される

Scenario: データ競合は検出される
  Given 競合を含むテストがある
  When  CI が `go test -race` を実行する
  Then  ジョブが失敗する

## 受け入れ基準
- [x] `.github/workflows/ci.yml` で `make check` 相当 + `go test -race -cover ./...` を実行
- [x] Go バージョンは go.mod の `go` 行から取得（`go-version-file`）
- [x] macOS / Linux のマトリクス
- [x] `staticcheck`（または golangci-lint の最小設定）と `govulncheck` を実行
- [x] Makefile に `lint` / `vuln` / `test-race` を追加し、CI は make 経由で呼ぶ（手元と CI の差をなくす）
- [x] AGENTS.md の「No CI workflows」記述を更新する

## テスト戦略
- E2E: テスト用ブランチで PR を作り成功・失敗の両ジョブを確認
- 統合: `make lint vuln test-race` をローカルで通す
- 単体: 対象外（設定のみ）

## 見積もり
2 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが実際の CI 実行で確認されている
- [x] `make check` がパスする
- [x] AGENTS.md / README のバッジ・手順更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`Makefile`、`AGENTS.md`（「No CI workflows」の記述）、`go.mod`（`go 1.25.14`）。

### 手順
1. `Makefile` に 3 つのターゲットを足し、`.PHONY` に追加する。

```make
## lint: staticcheck を実行
lint:
	$(GO) run honnef.co/go/tools/cmd/staticcheck@latest $(PKG)

## vuln: govulncheck を実行
vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest $(PKG)

## test-race: データ競合検出付きでテスト
test-race:
	$(GO) test $(GOFLAGS) -race -cover $(PKG)
```
   `go run pkg@version` は `go.mod` を変更しない。`go.mod` / `go.sum` に差分が出たら取り消す。
2. `.github/workflows/ci.yml` を作る。

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  check:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make check
      - run: make test-race
      - run: make lint
      - run: make vuln
```
3. `AGENTS.md` の「No CI workflows and no external dependencies」の行を、CI が存在する事実に合わせて書き換える（外部依存なしの方針は残す）。README の開発節に `make lint` / `make vuln` / `make test-race` を足す。
4. ローカルで `make check test-race` を実行して通すこと。`make lint` / `make vuln` はネットワークが要る。失敗した指摘は、コードを直す（`//lint:ignore` で黙らせない）。

### ユーザーへ確認が要る作業
workflow が実際に動くかの確認は push または PR が必要。push はユーザーの許可を得てから行う。許可がなければ「ローカルで make が通った。Actions での実行は未確認」と報告して終える。

### 注意
- `go-version-file: go.mod` を使い、Go のバージョンを workflow に直書きしない。
- 既存のコードに staticcheck の指摘が出たら、本 PBI の中で最小限に直す。大きな変更が要るなら止めて報告する。
