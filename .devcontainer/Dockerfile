FROM mcr.microsoft.com/vscode/devcontainers/go:0-1.16

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.42.0
RUN golangci-lint --version
