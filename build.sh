#!/bin/bash
# Copyright (c) 2026 Hygon Information Technology Co., Ltd.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Build binary (requires Linux host with CGO + HCU/dcgm headers and libs).
export GOPROXY="${GOPROXY:-https://goproxy.cn}"
export CGO_ENABLED=1
go mod tidy

BINARY=hcu-exporter
IMAGE_TAG="${IMAGE_TAG:-v3.0.0}"
IMAGE="harbor.sourcefind.cn:5443/hcu/admin/base/hcu-exporter:${IMAGE_TAG}"

go build -ldflags "-X 'main.version=${IMAGE_TAG}'" -o "${BINARY}" ./cmd/hcu-exporter

if [[ ! -f "${BINARY}" ]]; then
  echo "error: go build did not produce ${BINARY}; abort docker build" >&2
  exit 1
fi

# Package container image and save tar.
docker build -t "${IMAGE}" .
docker save -o "hcu-exporter-${IMAGE_TAG}.tar" "${IMAGE}"

echo "Done: ${IMAGE}"
