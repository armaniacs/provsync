BINARY := provsync
BIN_DIR := bin
PKG := ./...
GO ?= go
GOFLAGS ?=

.DEFAULT_GOAL := help

.PHONY: help build test test-v fmt fmt-check vet check install clean

## help: このヘルプを表示
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: バイナリを $(BIN_DIR)/$(BINARY) にビルド
build:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) .

## test: テストを実行
test:
	$(GO) test $(GOFLAGS) $(PKG)

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

## check: fmt-check + vet + test をまとめて実行(CI 向け)
check: fmt-check vet test

## install: go install でインストール
install:
	$(GO) install $(GOFLAGS) .

## clean: ビルド成果物を削除
clean:
	rm -rf $(BIN_DIR)