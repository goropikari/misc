.PHONY: all fmt test lint clean

all: fmt lint test

## fmt: コードのフォーマットを整え、自動修正を適用します
fmt:
	go fmt ./...
	go fix ./...
	golangci-lint run --fix

## test: 全てのテストを実行します
test:
	go test -v ./...

## lint: golangci-lint を実行して静的解析を行います
lint:
	golangci-lint run

## clean: ビルドキャッシュなどをクリアします
clean:
	go clean
