#!/usr/bin/env bash
# Test the inspector and upstream packages maintained by this fork.
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off GOENV=off GOFLAGS='' CGO_ENABLED=1
export GOTOOLCHAIN=go1.27.1+auto
tags="$(cat release/DEFAULT_BUILD_TAGS_OTHERS)"
ldflags="$(cat release/LDFLAGS)"
packages=(./service/heimspyinspector/... ./include ./cmd/sing-box ./common/ja3/... ./protocol/socks/...)
go test -race -mod=readonly "-tags=$tags,integration" "-ldflags=$ldflags" "${packages[@]}"
go vet -mod=readonly "-tags=$tags,integration" "${packages[@]}"
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 "-tags=$tags" ./service/heimspyinspector/...
