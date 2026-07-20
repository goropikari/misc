.PHONY: fmt lint mockgen server

fmt:
	golangci-lint run --fix

lint: gitleaks fmt
	golangci-lint run

mockgen:
	go generate ./...

server:
	go run ./cmd/server

gitleaks:
	gitleaks detect --no-banner --redact --source .
