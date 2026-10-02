# PBI: リリース自動化（GoReleaser / タグからの配布）

## ユーザーストーリー
利用者として、Go ツールチェーンなしでビルド済みバイナリを入手したい、なぜなら provsync のために Go を入れるのは敷居が高いから

## 優先度
- 順位: 09 / 19
- RICEスコア: 6.4（Reach=8 / Impact=1 / Confidence=80% / Effort=1）
- 根拠: 01（版の ldflags 注入）と 06（CI）に依存。配布が整うと init の導線（README の導入手順）が完結する

## BDD受け入れシナリオ
Scenario: タグ push でリリースが作られる
  Given CI が通った main がある
  When  `v0.3.0` のタグを push する
  Then  darwin / linux（amd64・arm64）のアーカイブと checksums が GitHub Release に添付される

Scenario: 配布バイナリの版が正しい
  Given リリースのバイナリをダウンロードした
  When  `provsync --version` を実行する
  Then  `provsync 0.3.0` が表示される

Scenario: タグと CHANGELOG の不整合は止める
  Given CHANGELOG に該当バージョンの節がない
  When  リリースを実行する
  Then  ジョブが失敗し、理由が表示される

## 受け入れ基準
- [x] `.goreleaser.yaml` と `.github/workflows/release.yml`
- [x] `CGO_ENABLED=0` の静的ビルド、`-trimpath`
- [x] checksums を添付
- [x] README の導入手順に `go install github.com/armaniacs/provsync@latest` とバイナリ入手の両方を記載
- [x] Homebrew tap は本 PBI の対象外（要望が出たら別 PBI）

## テスト戦略
- E2E: `goreleaser release --snapshot` をローカルで実行し成果物を確認
- 統合: プレリリースタグでの実リリース
- 単体: 対象外

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが確認されている
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`Makefile`、`CHANGELOG.md`、`internal/version/version.go`（01 で作成済み）、`.github/workflows/ci.yml`（06 で作成済み）。

### 手順
1. リポジトリ直下に `.goreleaser.yaml` を作る。`{{ .Version }}` は先頭の `v` が付かない値になり、`version.String()` の表記と一致する。

```yaml
version: 2
project_name: provsync
builds:
  - main: .
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w -X github.com/armaniacs/provsync/internal/version.Version={{ .Version }}
archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
checksum:
  name_template: checksums.txt
```
2. `.github/workflows/release.yml` を作る。

```yaml
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: CHANGELOG にバージョンの節があるか確認
        run: |
          v="${GITHUB_REF_NAME#v}"
          grep -q "^## \[$v\]" CHANGELOG.md || { echo "CHANGELOG.md に [$v] の節がありません"; exit 1; }
      - uses: goreleaser/goreleaser-action@v6
        with:
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
3. README の「インストール」節に、`go install github.com/armaniacs/provsync@latest` と、Releases からバイナリを入手する手順（`tar xzf` して PATH に置く）を書く。対応 OS は macOS / Linux。CHANGELOG `[Unreleased]` に追記。
4. ローカル確認: `goreleaser` が入っていれば `goreleaser release --snapshot --clean` を実行し、`dist/` の成果物で `--version` を確認する。入っていなければ「ローカル未確認」と報告する。

### ユーザーへ確認が要る作業
タグの作成・push はリリースを発生させる外部公開の操作。ユーザーの明示的な許可なしに実行しない。

### 注意
- Windows をビルド対象に入れない（非対応と決定済み）。
- Homebrew tap はこの PBI の対象外。
