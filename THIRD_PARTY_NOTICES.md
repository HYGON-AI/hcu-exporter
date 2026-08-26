# Third-Party Notices

本文件记录 `hcu-exporter` 仓库在构建与运行时依赖的第三方组件。本仓库**不内嵌**第三方源码副本（无 `vendor/`、`third_party/` 目录）；Go 依赖通过 Go Modules 在构建时拉取。

本项目许可证：`Apache-2.0`（见 [LICENSE](LICENSE)）。  
Copyright (c) 2026 Hygon Information Technology Co., Ltd.

## Direct Go module dependencies

以下为 `go.mod` 中的直接依赖。间接依赖见 `go.sum` / `go.mod` 的 `require` 间接段；许可证通常为 Apache-2.0 或 MIT（均在 HYGON 准入许可证范围内）。

| 项目 / 模块 | 仓库 URL | 固定版本（go.mod） | Copyright（上游声明） | 许可证 | 本地路径 | HYGON 修改 |
| --- | --- | --- | --- | --- | --- | --- |
| hcu-dcgm | https://github.com/HYGON-AI/hcu-dcgm | `github.com/HYGON-AI/hcu-dcgm/v3` `v3.1.1`（本地 `replace` → `../hcu-dcgm`） | Hygon Information Technology Co., Ltd. | Apache-2.0 | 构建时模块缓存；开发期 `replace` 至同级目录 `../hcu-dcgm` | 无（独立 HYGON 项目） |
| golang-set | https://github.com/deckarep/golang-set | `github.com/deckarep/golang-set/v2` `v2.8.0` | Dean Karn 等（见上游） | MIT | Go module cache | 无 |
| glog | https://github.com/golang/glog | `github.com/golang/glog` `v1.2.2` | Google LLC | Apache-2.0 | Go module cache | 无 |
| client_golang | https://github.com/prometheus/client_golang | `github.com/prometheus/client_golang` `v1.19.1` | The Prometheus Authors | Apache-2.0 | Go module cache | 无 |
| urfave/cli | https://github.com/urfave/cli | `github.com/urfave/cli/v2` `v2.27.1` | Jeremy Saenz 等（见上游） | MIT | Go module cache | 无 |
| grpc-go | https://github.com/grpc/grpc-go | `google.golang.org/grpc` `v1.64.0` | The gRPC Authors | Apache-2.0 | Go module cache | 无 |
| kubernetes/api | https://github.com/kubernetes/api | `k8s.io/api` `v0.30.2` | The Kubernetes Authors | Apache-2.0 | Go module cache | 无 |
| kubernetes/apimachinery | https://github.com/kubernetes/apimachinery | `k8s.io/apimachinery` `v0.30.2` | The Kubernetes Authors | Apache-2.0 | Go module cache | 无 |
| kubernetes/client-go | https://github.com/kubernetes/client-go | `k8s.io/client-go` `v0.30.2` | The Kubernetes Authors | Apache-2.0 | Go module cache | 无 |
| kubernetes/kubelet | https://github.com/kubernetes/kubelet | `k8s.io/kubelet` `v0.30.2` | The Kubernetes Authors | Apache-2.0 | Go module cache | 无 |

## Runtime / packaging notes

- 运行时依赖宿主机或容器内的 HCU 驱动库（如 `/opt/hyhal` 下的 `libhydmi.so`、`librocm_smi64.so`），驱动库本身**不随本仓库分发**，亦不作为本仓库第三方源码嵌入。
- Docker 基础镜像 `ubuntu:22.04` 及其系统包按各自上游许可证分发；本仓库 `Dockerfile` 仅声明构建与打包方式。
- Grafana 仪表盘 JSON（`grafana/*.json`）为本项目配置资产，不是第三方源码嵌入。

## Compliance boundary

- 本仓库源码树中的 Go / Shell / Dockerfile / Helm / 静态 YAML 为 HYGON 原创或维护内容，适用 `Apache-2.0` 与 HYGON Copyright 文件头（见各文件 SPDX 声明）。
- 不在本仓库内机械添加第三方 Copyright；第三方义务以各模块上游声明及本清单为准。
- 若公开发布前取消 `go.mod` 中的 `replace`，应以已发布的固定模块版本为准，并同步更新本清单中的版本字段。
- 本仓库不包含 NCSA 等需额外法务准入的嵌入式第三方头文件；相关依赖若出现在 `hcu-dcgm` 中，由该项目单独登记与审批。
