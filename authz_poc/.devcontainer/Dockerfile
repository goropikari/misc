FROM mcr.microsoft.com/devcontainers/go:1.26-trixie

RUN mkdir -p /home/vscode/.codex \
    && chown -R vscode:vscode /home/vscode/.codex

RUN apt-get update && apt-get upgrade -y && \
    apt-get install -y \
        ripgrep \
        jq
