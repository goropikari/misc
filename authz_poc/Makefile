.PHONY: all fmt fix lint install install-codex install-dprint install-gitleaks install-typos install-dev-tools

LOCAL_BIN ?= $(HOME)/.local/bin

fmt: fix
	golangci-lint fmt
	dprint fmt

fix:
	golangci-lint run --fix ./...

lint: gitleaks typos fmt
	golangci-lint run ./...

typos:
	typos

gitleaks:
	gitleaks detect --no-banner --redact --source .

install:
	go install ./cmd/gomut

install-codex:
	@set -eu; \
	if ! command -v codex >/dev/null 2>&1; then \
		curl -fsSL https://chatgpt.com/codex/install.sh | sh; \
	fi; \
	codex --version

install-dprint:
	@set -eu; \
	mkdir -p '$(DPRINT_INSTALL)/bin'; \
	if ! command -v dprint >/dev/null 2>&1; then \
		curl -fsSL https://dprint.dev/install.sh | sh; \
	fi
	dprint --version

install-gitleaks:
	@set -eu; \
	mkdir -p '$(LOCAL_BIN)'; \
	if ! command -v gitleaks >/dev/null 2>&1; then \
		tmpdir="$$(mktemp -d)"; \
		url="$$(curl -fsSL https://api.github.com/repos/gitleaks/gitleaks/releases/latest | grep -Eo '"browser_download_url": *"[^"]+"' | cut -d'"' -f4 | grep -E 'linux.*(x64|amd64).*\.tar\.gz$$' | head -n1)"; \
		[ -n "$$url" ]; \
		curl -fsSL "$$url" -o "$$tmpdir/gitleaks.tar.gz"; \
		tar -xzf "$$tmpdir/gitleaks.tar.gz" -C "$$tmpdir"; \
		binary="$$(find "$$tmpdir" -type f -name gitleaks -perm -u+x | head -n1)"; \
		install -m 755 "$$binary" '$(LOCAL_BIN)/gitleaks'; \
		rm -rf "$$tmpdir"; \
	fi

install-typos:
	@set -eu; \
	mkdir -p '$(LOCAL_BIN)'; \
	if ! command -v typos >/dev/null 2>&1; then \
		tmpdir="$$(mktemp -d)"; \
		url="$$(curl -fsSL https://api.github.com/repos/crate-ci/typos/releases/latest | grep -Eo '"browser_download_url": *"[^"]+x86_64-unknown-linux-musl\.tar\.gz"' | cut -d'"' -f4 | head -n1)"; \
		[ -n "$$url" ]; \
		curl -fsSL "$$url" -o "$$tmpdir/typos.tar.gz"; \
		tar -xzf "$$tmpdir/typos.tar.gz" -C "$$tmpdir"; \
		binary="$$(find "$$tmpdir" -type f -name typos -perm -u+x | head -n1)"; \
		install -m 755 "$$binary" '$(LOCAL_BIN)/typos'; \
		rm -rf "$$tmpdir"; \
	fi

install-dev-tools: install-codex install-dprint install-gitleaks install-typos
