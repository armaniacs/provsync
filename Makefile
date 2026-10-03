BINARY := provsync
BIN_DIR := bin
PKG := ./...
GO ?= go
GOFLAGS ?=
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/armaniacs/provsync/internal/version.Version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help all build test test-all tui-test test-v test-race fuzz fmt fmt-check vet lint vuln check install tui-build tui-check docs docs-serve clean

## help: このヘルプを表示
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: バイナリを $(BIN_DIR)/$(BINARY) にビルド
build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

## all: CLI と TUI の両方をビルド
all: build tui-build

## test: テストを実行
test:
	$(GO) test $(GOFLAGS) $(PKG)

## tui-test: TUI モジュールのテストを実行
tui-test:
	cd tui && $(GO) test $(GOFLAGS) ./...

## test-all: CLI と TUI の両方のテストを実行
test-all: test tui-test

## test-v: テストを詳細表示
test-v:
	$(GO) test $(GOFLAGS) -v $(PKG)

## fmt: コードを整形
fmt:
	gofmt -w .

## fmt-check: 整形崩れがないか確認
fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "not formatted:"; echo "$$out"; exit 1; fi

## vet: go vet を実行
vet:
	$(GO) vet $(GOFLAGS) $(PKG)

## lint: staticcheck を実行
lint:
	$(GO) run honnef.co/go/tools/cmd/staticcheck@latest $(PKG)

## vuln: govulncheck を実行
vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest $(PKG)

## test-race: データ競合検出付きでテスト
test-race:
	$(GO) test $(GOFLAGS) -race -cover $(PKG)

## fuzz: StripJSONC の短時間ファズテスト
fuzz:
	$(GO) test -fuzz=FuzzStripJSONC -fuzztime=20s ./internal/jsonc

## check: fmt-check + vet + test をまとめて実行(CI 向け)
check: fmt-check vet test

## tui-build: TUI バイナリを tui/ にビルド
tui-build:
	cd tui && $(GO) build -o provsync-tui .

## tui-check: TUI モジュールのビルドとテスト
tui-check:
	cd tui && $(GO) build ./... && $(GO) test ./...

## install: go install でインストール
install:
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" .

## clean: ビルド成果物を削除
clean:
	rm -rf $(BIN_DIR)

## docs: ドキュメントサイトを site/ へビルド
docs: venv-check
	$(VENV)/bin/mkdocs build --strict --site-dir site

## docs-serve: ドキュメントサイトをローカルプレビュー
docs-serve: venv-check
	$(VENV)/bin/mkdocs serve

## venv-check: ドキュメント用 venv の有無を確認
VENV := .venv
venv-check:
	@test -x "$(VENV)/bin/mkdocs" || { \
		echo "docs 用の仮想環境がありません。次を実行してください:"; \
		echo "  python3 -m venv $(VENV) && $(VENV)/bin/pip install -r requirements.txt"; \
		exit 1; }