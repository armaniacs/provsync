# PBI: バージョン番号を表示する

## ユーザーストーリー
provsync の利用者として、`provsync --version` と引数なし実行でバージョンを確認したい、なぜなら不具合報告やアップグレード判断の際に、手元のバイナリがどの版か即座に分かる必要があるから

## 優先度
- 順位: 01 / 19
- RICEスコア: 40（Reach=10 / Impact=1 / Confidence=100% / Effort=0.25）
- 根拠: 全利用者に効き、実装が極小。09（リリース自動化）が ldflags によるバージョン注入に依存するため先行させる

## BDD受け入れシナリオ
Scenario: --version でバージョンを表示する
  Given provsync をビルド済みである
  When  `provsync --version` を実行する
  Then  `provsync 0.2.1` のように版が 1 行で表示され、終了コードは 0 である

Scenario: 引数なし実行のヘッダにバージョンが出る
  Given provsync をビルド済みである
  When  `provsync` を引数なしで実行する
  Then  使い方の先頭にバージョンが表示される

Scenario: go install で入れたバイナリでも版が分かる
  Given ldflags なしで `go install` したバイナリがある
  When  `provsync --version` を実行する
  Then  モジュールのバージョン、取得できなければ `dev` が表示される

## 受け入れ基準
- [x] `--version` と `version` サブコマンドの両方が使える
- [x] 版の決定順は ldflags 注入値、`runtime/debug.ReadBuildInfo` のモジュール版、`dev` の順
- [x] 版が取得できる場合は VCS リビジョンも併記できる（任意）
- [x] Makefile の build が `-ldflags "-X ...=$(VERSION)"` で `git describe` を注入する
- [x] 外部依存を追加しない（標準ライブラリのみ）

## テスト戦略
- E2E: ビルドしたバイナリで `--version` の出力と終了コードを確認
- 統合: `cli.Run` で `--version` / `version` / 引数なしの出力を検証
- 単体: 版解決関数の優先順位（注入値あり / BuildInfo のみ / どちらもなし）

## 見積もり
1 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/cli/cli.go` の `Run` / `options` / `registerFlags` / `usage`、`Makefile`、`main.go`。

### 手順
1. 新規パッケージ `internal/version/version.go` を作る。

```go
// Package version はバイナリのバージョン文字列を解決する。
package version

import (
	"runtime/debug"
	"strings"
)

// Version はビルド時に -ldflags "-X ...version.Version=1.2.3" で注入される。
var Version = ""

func String() string {
	info, ok := debug.ReadBuildInfo()
	return resolve(Version, info, ok)
}

// resolve は注入値、BuildInfo のモジュール版、"dev" の順に採用する。
func resolve(injected string, info *debug.BuildInfo, ok bool) string {
	if injected != "" {
		return strings.TrimPrefix(injected, "v")
	}
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}
```

2. `internal/version/version_test.go` にテーブルテスト（`resolve` を直接呼ぶ）: 注入値 `"v0.2.1"` → `"0.2.1"`、注入なし + BuildInfo `v0.3.0` → `"0.3.0"`、注入なし + `(devel)` → `"dev"`、`ok=false` → `"dev"`。
3. `internal/cli/cli.go`:
   - `options` に `version bool` を足し、`registerFlags` に `fs.BoolVar(&o.version, "version", o.version, "バージョンを表示")`。
   - `Run` で `fs.Parse` の直後、`pos := fs.Args()` の後に追加: `if opts.version { fmt.Fprintf(out, "provsync %s\n", version.String()); return nil }`。
   - `switch cmd` に `case "version": fmt.Fprintf(o.out, ...)` を足す（`opts.out` を使う）。
   - `usage` の最初の行を `provsync <version>` にする（`fmt.Fprintf(out, "provsync %s\n\n", version.String())` を先頭に出す）。
4. `Makefile`: 先頭付近に次を足し、`build` と `install` に `-ldflags "$(LDFLAGS)"` を付ける。

```make
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/armaniacs/provsync/internal/version.Version=$(VERSION)
```
   `build`: `$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .`
5. `cli_test.go` に追加: `TestVersionFlag`（`mustRun(t, f.root, "--version")` の出力が `provsync ` で始まり改行 1 つ）、`TestNoArgsShowsVersion`（引数なしの出力に `provsync ` が含まれる）。

### 確認コマンド
`make build && ./bin/provsync --version`（`provsync 0.2.1-...` のように出る）。`go run . --version`（ldflags なしなので `provsync dev`）。

### 注意
- `Version` の初期値を `"dev"` にしない（`""` にする。BuildInfo の分岐が死ぬため）。
- 既存の usage の文言を検査するテストがあれば、先頭行追加に合わせて直す。それ以外の既存テストは変えない。
- README のコマンドリファレンスと CHANGELOG `[Unreleased]` の `Added` に書く。
