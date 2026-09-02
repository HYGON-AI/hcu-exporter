# HCU-Exporter

HCU-Exporter 是面向海光 HCU 的 Prometheus 指标导出器。它通过 [hcu-dcgm](https://github.com/HYGON-AI/hcu-dcgm)（Go 模块名 `github.com/HYGON-AI/hcu-dcgm/v3`）调用 DCGM 接口，周期性采集物理 HCU 与 vHCU 的运行指标，并在 `/metrics` 端点以 Prometheus 格式对外暴露。

## 目录

- [功能特性](#功能特性)
- [架构概览](#架构概览)
- [前置条件](#前置条件)
- [快速开始](#快速开始)
- [源码编译](#源码编译)
- [配置参考](#配置参考)
- [采集成本与 metrics-level](#采集成本与-metrics-level)
- [指标与标签自定义](#指标与标签自定义)
- [监控指标](#监控指标)
- [Kubernetes Pod 关联](#kubernetes-pod-关联)
- [Prometheus 与 Grafana](#prometheus-与-grafana)
- [项目结构](#项目结构)
- [常见问题](#常见问题)
- [License](#license)
- [Third-Party](#third-party)

## 功能特性

- **物理 HCU 与 vHCU 指标采集**：覆盖温度、功耗、利用率、显存、PCIe/DF/Hylink 带宽、ECC 错误等
- **细粒度利用率指标**：支持 CU 瞬时利用率、采样窗口利用率、Wave 采样利用率及 Shader Engine（SE）利用率
- **Kubernetes 集成**（可选）：通过 Kubelet Pod Resources API 与 Pod Informer，将设备指标与 Pod / 容器关联
- **指标按需启用**：通过 `--metrics-level` 按采集成本分档，或通过 `--enable-metrics` 精确指定监控项
- **指标与标签自定义**：通过 `--metrics-define` / `--label-define` 重命名导出的指标名与 Label 名，适配既有监控体系
- **灵活部署**：支持裸机、Docker 与 Kubernetes（静态清单 / Helm Chart）
- **访问控制**：支持对 `/metrics` 配置 IP / CIDR 白名单
- **Hylink 明细模式**：`--hylink-detail` 可输出每条链路的独立指标
- **采集健康检测**：若超过 60 秒未更新采集时间戳，进程自动退出，便于外部（如 DaemonSet）重启

## 架构概览

```
┌─────────────────┐    DCGM API     ┌──────────────┐
│  hcu-exporter   │ ◄──────────────►│   hcu-dcgm   │
│  (本仓库)        │                 │ (CGO 绑定层)  │
└────────┬────────┘                 └──────┬───────┘
         │                                 │
         │  HTTP /metrics                    │ libhydmi.so
         ▼                                 │ librocm_smi64.so
┌─────────────────┐                        ▼
│   Prometheus    │                 ┌──────────────┐
└─────────────────┘                 │  HCU 硬件/驱动 │
                                    └──────────────┘

Kubernetes 环境（connect-k8s=true 且 Kubelet Socket 可用时）额外依赖：
  · Kubelet Pod Resources Socket：/var/lib/kubelet/pod-resources/kubelet.sock
  · Kubernetes API：Pod Informer，监听本节点 HCU Pod 变化并刷新设备关联信息
  · /etc/vdev/dynamic：动态 vHCU（hcunum）场景下的设备-Pod 映射
```

程序启动后主要执行以下流程：

1. 解析 `--metrics-define` / `--label-define` 等配置，初始化 DCGM
2. 创建独立的 Prometheus Registry，按 `--metrics-level` / `--enable-metrics` 注册指标（应用自定义展示名与 Label 名）
3. 后台 goroutine 按 `--pulse` 间隔采集 HCU / vHCU 数据并更新指标
4. 若启用 K8s 集成，启动 Pod Informer 监听本节点 Pod 变化，按需刷新设备与 Pod 的映射
5. 启动 HTTP 服务，在 `/metrics` 暴露指标（可选 IP 白名单中间件）

## 前置条件

### 运行环境

| 项目     | 要求                                                         |
| -------- | ------------------------------------------------------------ |
| 操作系统 | Linux（依赖 CGO 与 HCU 驱动库，**不支持在 Windows 上编译/运行**） |
| Go 版本  | 1.25+（编译时；`go.mod` 当前要求）                           |
| HCU 驱动 | 已安装，建议版本 ≥ 6.2.26                                    |
| 动态库   | `libhydmi.so`（随 HCU 驱动）、`librocm_smi64.so`（随 DTK 或位于 `/opt/hyhal/lib`） |

### 动态库配置

**方式一（推荐）：安装驱动与 DTK**

1. 安装 HCU 驱动（包含 `libhydmi.so`）
2. 安装 DTK 并执行 `source <dtk_dir>/env.sh`（包含 `librocm_smi64.so`）

**方式二：手动指定库路径**

将所需 `.so` 文件放置到固定目录（如 `/opt/hyhal/lib`），并设置：

```bash
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/opt/hyhal/lib
```

容器部署时通常挂载宿主机的 `/opt/hyhal` 目录（见下方部署示例）。

## 快速开始

### 裸机部署

```bash
./hcu-exporter --port 16080
```

按采集成本选用档位（多卡或短 scrape 间隔建议 `low`/`medium`）：

```bash
./hcu-exporter --port 16080 --metrics-level=medium
```

无需关联 Kubernetes Pod 信息时：

```bash
./hcu-exporter --port 16080 --connect-k8s=false
```

验证指标：

```bash
curl localhost:16080/metrics
```

### Docker 部署

```bash
docker run --name hcu-exporter -d --privileged \
  --device=/dev/kfd \
  --device=/dev/mkfd \
  --device=/dev/dri \
  -v /etc/hostname:/etc/hostname \
  -v /etc/vdev:/etc/vdev \
  -v /opt/hyhal:/opt/hyhal \
  -p 16080:16080 \
  -e CONNECT_K8S=false \
  -e METRICS_LEVEL=medium \
  hcu-exporter:latest
```

也可在镜像名后追加命令行参数（与环境变量等价，命令行优先）：

```bash
docker run --name hcu-exporter -d --privileged \
  --device=/dev/kfd --device=/dev/mkfd --device=/dev/dri \
  -v /etc/hostname:/etc/hostname -v /etc/vdev:/etc/vdev -v /opt/hyhal:/opt/hyhal \
  -p 16080:16080 -e CONNECT_K8S=false \
  hcu-exporter:latest \
  --metrics-level=low --pulse=5
```

也可使用项目提供的启动脚本 `deployment/docker/hcu-exporter-start.sh`，支持环境变量 `ALLOWED_IPS`、`CONNECT_K8S`、`METRICS_LEVEL`：

```bash
export CONNECT_K8S=false
export METRICS_LEVEL=medium
./deployment/docker/hcu-exporter-start.sh
```

### Kubernetes 部署

推荐使用 **DaemonSet** 在每个 HCU 节点上部署。

**静态清单**

```bash
kubectl apply -f deployment/static/hcu-exporter.yaml
```

清单包含 DaemonSet、Service、ServiceAccount 与 RBAC，默认：

- `nodeSelector: hygon.com/hcu: "true"` 选择 HCU 节点
- `hostNetwork: true`，挂载 `/var/lib/kubelet`、`/dev`、`/etc/vdev`、`/opt/hyhal`
- Prometheus 自动发现注解：`prometheus.io/scrape`、`prometheus.io/port`、`prometheus.io/path`

**Helm Chart**

Chart 位于 `deployment/helm/hcu-exporter/`，通过 `values.yaml` 调整镜像、端口、环境变量等：

```bash
helm install hcu-exporter ./deployment/helm/hcu-exporter
```

Helm 模板在 `connectK8s=true` 时自动挂载 Kubelet 目录并创建 Pod 读取 RBAC；设为 `false` 时跳过 K8s 相关配置，适用于特殊裸机场景。

## 源码编译

### 依赖仓库

本项目 Go 模块为 `github.com/HYGON-AI/hcu-exporter/v3`，依赖 [hcu-dcgm](https://github.com/HYGON-AI/hcu-dcgm)。建议将两个仓库克隆为同级目录：

```
your-workspace/
├── hcu-exporter/    # 本仓库
└── hcu-dcgm/        # DCGM Go 绑定库
```

`go.mod` 中通过 `replace` 指向本地路径：

```go
replace github.com/HYGON-AI/hcu-dcgm/v3 => ../hcu-dcgm
```

若 `hcu-dcgm` 路径不同，请相应修改 `replace` 指令。

### 编译步骤

```bash
export CGO_ENABLED=1
export GOPROXY=https://goproxy.cn,direct

cd hcu-exporter
go mod tidy
go build -ldflags "-X 'main.version=<版本号>'" -o hcu-exporter ./cmd/hcu-exporter
```

或使用项目脚本（编译 + 构建镜像 + 导出 tar 包）：

```bash
./build.sh
```

### 构建 Docker 镜像

```bash
# 先将编译产物 hcu-exporter 置于项目根目录（与 Dockerfile COPY 路径一致）
docker build -t hcu-exporter:latest .
```

## 配置参考

命令行参数与环境变量一一对应，**命令行优先级高于环境变量**。

| 参数                   | 环境变量              | 默认值               | 说明                                                                 |
| ---------------------- | --------------------- | -------------------- | -------------------------------------------------------------------- |
| `--pulse`              | `PULSE`               | `10`                 | 指标采集间隔（秒）                                                   |
| `--port`               | `HCU_EXPORTER_LISTEN` | `16080`              | HTTP 监听端口                                                        |
| `--metrics-level`      | `METRICS_LEVEL`       | `high`               | 按采集成本启用指标档位：`low` / `medium` / `high`（详见下节）        |
| `--enable-metrics`     | `ENABLE_METRICS`      | 空                   | 逗号分隔的**内部指标名**列表；**非空时优先生效，覆盖** `--metrics-level` |
| `--metrics-define`     | `METRICS_DEFINE`      | 空                   | JSON 映射：内部指标名 → 导出展示名（详见[指标与标签自定义](#指标与标签自定义)） |
| `--label-define`       | `LABEL_DEFINE`        | 空                   | JSON 全局 Label 重命名映射（详见[指标与标签自定义](#指标与标签自定义)） |
| `--sample-duration-ms` | `SAMPLE_DURATION_MS`  | `1000`               | 采样类利用率指标的采样窗口时长（毫秒）；仅 `high` 档中相关指标生效     |
| `--kube-config`        | `KUBE_CONFIG`         | `/root/.kube/config` | Kubeconfig 路径；不存在时使用 InCluster 配置                         |
| `--connect-k8s`        | `CONNECT_K8S`         | `true`               | 是否连接 K8s 采集 Pod 关联信息                                       |
| `--hylink-detail`      | `HYLINK_DETAIL`       | `false`              | 是否输出 Hylink 每条链路的明细指标                                   |
| `--ips`                | `ALLOWED_IPS`         | 空（不限制）         | 允许访问 `/metrics` 的 IP 或 CIDR，逗号分隔                          |
| `--stderrthreshold`    | `LOG_THRESHOLD`       | `INFO`               | 日志级别：`INFO` / `WARNING` / `ERROR`                               |
| `--log-verbose`        | `LOG_VERBOSE`         | `2`                  | 详细日志级别（0–10）                                                 |
| `--alsologtostderr`    | `LOG_OUTPUT`          | `true`               | 是否输出日志到 stderr                                                |

**节点名称**：指标 `node` 标签优先读取环境变量 `NODE_NAME`（K8s 部署时由 Downward API 注入），否则读取 `/etc/hostname`。

**示例：按采集成本选用档位（二进制）**

```bash
# 仅低成本直接查询（推荐短 scrape 间隔 / 多卡节点）
./hcu-exporter --metrics-level=low --pulse=5

# 低成本 + DF/Hylink/进程/健康检查
./hcu-exporter --metrics-level=medium

# 全部指标（默认，含约 1s 窗口类）
./hcu-exporter --metrics-level=high

# 也可用环境变量（命令行优先）
export METRICS_LEVEL=medium
./hcu-exporter --port 16080 --connect-k8s=false
```

**示例：按采集成本选用档位（Docker）**

```bash
# 环境变量
docker run --name hcu-exporter -d --privileged \
  --device=/dev/kfd --device=/dev/mkfd --device=/dev/dri \
  -v /etc/hostname:/etc/hostname -v /etc/vdev:/etc/vdev -v /opt/hyhal:/opt/hyhal \
  -p 16080:16080 \
  -e CONNECT_K8S=false \
  -e METRICS_LEVEL=low \
  hcu-exporter:latest

# 或镜像后追加参数
docker run --name hcu-exporter -d --privileged \
  --device=/dev/kfd --device=/dev/mkfd --device=/dev/dri \
  -v /etc/hostname:/etc/hostname -v /etc/vdev:/etc/vdev -v /opt/hyhal:/opt/hyhal \
  -p 16080:16080 -e CONNECT_K8S=false \
  hcu-exporter:latest \
  --metrics-level=medium
```

**示例：精确指定指标（覆盖 metrics-level）**

```bash
./hcu-exporter \
  --enable-metrics=hcu_temp,hcu_utilizationrate,hcu_usedmemory_bytes \
  --ips=127.0.0.1,10.0.0.0/8
```

**示例：自定义指标名与 Label 名**

```bash
./hcu-exporter \
  --metrics-define='{"hcu_ce_count":"ce_count","hcu_compute_unit_count":"compute_unit_count"}' \
  --label-define='{"block_type":"b_type","hcu_pod_name":"pod_name","hcu_pod_namespace":"namespace","device_id":"uuid"}'
```

## 采集成本与 metrics-level

部分 DCGM / RSMI 接口为**采样窗口型**（需等待约 1 秒统计周期），另一些为**直接读取**。耗时参考来自 `hy-smi` 进程墙钟实测（含 CLI 开销）；exporter 经 cgo 直调通常略快，但窗口型等待行为相近。

档位为**累加**关系：`medium ⊇ low`，`high ⊇ medium`。默认 `high` 保持与历史「启用全部指标」行为一致。

| 档位 | 单卡额外等待粗估 | 适用场景 | 包含内容 |
| ---- | ---------------- | -------- | -------- |
| `low` | 通常数十毫秒量级（多项直接读取叠加） | 短 scrape 间隔、卡数多、只要核心运行态 | 温控 / 功耗 / 时钟 / 显存 / 瞬时利用率 `hcu_utilizationrate` / ECC / 风扇 / 拓扑 / UMC / vHCU 等直接查询 |
| `medium` | 在 low 基础上再增加约数十～数百毫秒 | 需要总线带宽、进程占用、健康状态 | low + DF/Hylink/XHCL 带宽、`hcu_health_status`、进程类 `hcu_process_*` |
| `high` | 在 medium 基础上，**每卡每轮**可能再增加约 **1～数秒** | 需要 PCIe 吞吐、last-second util、采样窗口利用率 | medium + 下表「高成本」指标 |

### 高成本指标（仅 `high`）

| 内部指标名 | dcgm API | hy-smi 参考 | 文档中位耗时 | 说明 |
| ---------- | -------- | ----------- | -----------: | ---- |
| `hcu_pciebw_mb` / `hcu_pcie_sent_mb` / `hcu_pcie_receive_mb` | `PcieBw` | `--showbw` | ~1088 ms | 固定约 1s 吞吐窗口；三指标合并一次调用 |
| `hcu_util_percent` | `DevBusyPercent` | `--showhcuutil` | ~1025 ms | last second busy% |
| `hcu_cu_usage` | `HCUCuUsage` | `--showcuutil` | ~1074 ms | last second CU 利用率 |
| `hcu_sampled_usage` / `hcu_cu_sampled_usage` / `hcu_wave_sampled_usage` | `HCU*SampledUsage` | 无独立 CLI | ≈ `--sample-duration-ms`（默认 1000） | 调用方指定采样窗口 |
| `hcu_cu_util` / `hcu_wave_util` | `DevCuUtil` / `DevWaveUtil` | — | ≈ `--sample-duration-ms` | 同上 |

> 若同时启用多项高成本指标，等待会**叠加**（PCIe 三项除外，只调一次 `PcieBw`）。可将 `--sample-duration-ms` 调小以降低采样类耗时，并重新压测。

### 中成本补充（`medium` / `high`）

| 内部指标名 | dcgm API | hy-smi 参考 | 文档中位耗时 |
| ---------- | -------- | ----------- | -----------: |
| `hcu_df_bw_*` | `DFBandwidth` | `--showdfbw` | ~54 ms |
| `hcu_hylink_*` / `hcu_xhcl_bw` | `HyLinkStatusByHcuId` / `XHCLBandwidth` | `--showxhclbw` 等 | 数毫秒～采样 delay（环境相关） |
| `hcu_health_status` | `HCUHealthCheck` | `--healthcheck` | ~44 ms |
| `hcu_process_*` | `ProcessHCUInfo` 等 | `--showpids` | ~36 ms（进程多时上升） |

### 低成本示例（`low` 及更高档均包含）

温度、功耗、`hcu_utilizationrate`（`HCUUse`，约 13 ms，**不是** 1s 的 `hcu_util_percent`）、显存、时钟、ECC、风扇、PCIe 宽度/时钟/重放计数、NUMA、vHCU 等。完整列表见源码 `pkg/util/util.go` 中 `metricsLevelLow`。

### 与 `--enable-metrics` 的关系

| 配置 | 行为 |
| ---- | ---- |
| 仅 `--metrics-level` | 按档位启用预设指标集 |
| 仅 `--enable-metrics` | 精确启用列出的内部指标名 |
| 两者同时配置 | **`--enable-metrics` 覆盖档位**（启动时打 Warning） |

```bash
# 档位方式
./hcu-exporter --metrics-level=medium

# 精确列表（即使写了 metrics-level=low 也会被覆盖）
./hcu-exporter --metrics-level=low --enable-metrics=hcu_temp,hcu_pciebw_mb
```

## 指标与标签自定义

Exporter 支持在启动时重命名导出的指标名与 Label 名，便于对接已有 Prometheus / Grafana 命名规范。

### `--metrics-define`：重命名指标

JSON 格式为 `{"内部指标名": "展示名"}`：

- **Key** 必须是程序内置的指标名（如 `hcu_ce_count`），与 `--enable-metrics` 使用同一套名称
- **Value** 为 `/metrics` 中实际暴露的 Prometheus 指标名
- 未出现在 JSON 中的指标保持原名
- 所有最终展示名（含未改名的默认名）必须全局唯一；配置非法时启动失败

```json
{
  "hcu_ce_count": "ce_count",
  "hcu_compute_unit_count": "compute_unit_count"
}
```

上例中，`hcu_ce_count` 导出为 `ce_count`，`hcu_compute_unit_count` 导出为 `compute_unit_count`，其余指标不变。

### `--label-define`：重命名 Label

JSON 格式为 `{"原label名": "展示label名"}`，**全局生效**：

- 对所有指标，只要包含配置中的原 Label 名，导出时统一替换为展示名
- 未出现在配置中的 Label 保持原名
- 展示 Label 名必须全局唯一；若某指标在重命名后出现 Label 重名，启动失败

```json
{
  "block_type": "b_type",
  "hcu_pod_name": "pod_name",
  "hcu_pod_namespace": "namespace",
  "device_id": "uuid"
}
```

上例中，所有包含 `device_id` 的指标都会将 Label 导出为 `uuid`；`hcu_ce_count` 的 `block_type` 会导出为 `b_type`，以此类推。

### 配置关系说明

| 配置项              | 使用的指标名称体系 | 说明                                       |
| ------------------- | ------------------ | ------------------------------------------ |
| `--metrics-level`   | 内部指标名预设集   | 按采集成本启用 low / medium / high 档指标  |
| `--enable-metrics`  | 内部指标名         | 精确控制采集与注册哪些指标（覆盖档位）     |
| `--metrics-define`  | 内部指标名 → 展示名 | 控制 `/metrics` 中的指标名                 |
| `--label-define`    | 原 Label 名 → 展示名 | 全局控制 `/metrics` 中的 Label 名        |

采集逻辑始终基于内部指标名运行；仅在对外暴露时应用展示名与 Label 重命名。

## 监控指标

Exporter 使用独立 Registry，仅暴露由 `--metrics-level` 或 `--enable-metrics` 启用的指标。未识别的指标名称会在启动时打印警告并跳过。高成本指标仅在 `high` 档（或显式列入 `--enable-metrics`）时采集，详见[采集成本与 metrics-level](#采集成本与-metrics-level)。

下表列出所有内置指标的**内部名称**（即默认导出名称）。若配置了 `--metrics-define`，实际 `/metrics` 中的名称以展示名为准。

### 物理 HCU 指标

| 内部指标名                         | 说明                                                 |
| ---------------------------------- | ---------------------------------------------------- |
| `hcu_temp`                         | 温度                                                 |
| `hcu_power_usage`                  | 当前功耗                                             |
| `hcu_powercap`                     | 功耗上限                                             |
| `hcu_sclk`                         | 核心时钟频率                                         |
| `hcu_mclk`                         | 内存时钟频率                                         |
| `hcu_utilizationrate`              | 计算单元利用率（瞬时，`HCUUse`；低成本）             |
| `hcu_cu_usage`                     | CU 瞬时利用率（%；**高成本**，约 1s 窗口）           |
| `hcu_sampled_usage`                | 采样窗口内 HCU 利用率（%；**高成本**，由 `--sample-duration-ms` 控制） |
| `hcu_cu_sampled_usage`             | 采样窗口内 CU 平均利用率（%；**高成本**）            |
| `hcu_wave_sampled_usage`           | 采样窗口内 Wave 平均利用率（%；**高成本**）          |
| `hcu_se_usage`                     | Shader Engine 瞬时利用率（%），按 `se_id` 分引擎输出 |
| `hcu_cu_util`                      | CU wave 占用周期占比（0~1；**高成本**，窗口同 `--sample-duration-ms`） |
| `hcu_wave_util`                    | Wave 驻留占比（0~1；**高成本**）                    |
| `hcu_usedmemory_bytes`             | 已用显存                                             |
| `hcu_memorycap_bytes`              | 显存总量                                             |
| `hcu_available_memory_bytes`       | 可用显存（对应 hy-smi `Available memory size`）      |
| `hcu_vram_percent`                 | 显存使用百分比（对应 `VRAM%` / `HCU memory use (%)`） |
| `hcu_util_percent`                 | HCU 利用率（对应 `HCU util in last second`；**高成本**，约 1s） |
| `hcu_temp_mem`                     | 显存温度（℃，对应 `Temperature (Sensor mem)`）       |
| `hcu_temp_board`                   | 板卡温度（℃；CLI 无独立 board 标签）                 |
| `hcu_sensor_temp`                  | 传感器温度（℃）；`sensor_type`=`edge`/`junction`/`mem`/`core` |
| `hcu_sensor_temp_max`              | 上述四路传感器温度上限（℃），`GetTempBySensor`+`RSMI_TEMP_MAX` |
| `hcu_sensor_temp_critical`         | 上述四路传感器临界温度（℃），`GetTempBySensor`+`RSMI_TEMP_CRITICAL` |
| `hcu_temp_edge_current`            | Sensor edge 当前温度（℃），`GetTempByMetric`+`RSMI_TEMP_CURRENT` |
| `hcu_temp_edge_critical`           | Sensor edge 临界温度（℃），`GetTempByMetric`+`RSMI_TEMP_CRITICAL` |
| `hcu_temp_edge_emergency`          | Sensor edge 紧急温度（℃），`GetTempByMetric`+`RSMI_TEMP_EMERGENCY` |
| `hcu_throttle`                     | 降频标志（1=生效），标签 `throttle_type`             |
| `hcu_sclk_max`                     | GFX 最大频率（MHz）                                  |
| `hcu_mclk_max`                     | 显存最大频率（MHz）                                  |
| `hcu_overdrive`                    | GFX Overdrive（对应 `OverDriver value (%)`）         |
| `hcu_memory_overdrive`             | 显存 Overdrive（对应 `Memory OverDriver value (%)`） |
| `hcu_powercap_range_max`           | 可配置功耗范围上限（W；非 `--showmaxpower`）         |
| `hcu_powercap_range_min`           | 可配置功耗范围下限（W）                              |
| `hcu_pciebw_mb`                    | PCIe 总带宽（发送 + 接收；**高成本**，约 1s 窗口）   |
| `hcu_pcie_sent_mb`                 | PCIe 发送带宽（**高成本**）                          |
| `hcu_pcie_receive_mb`              | PCIe 接收带宽（**高成本**）                          |
| `hcu_pcie_width`                   | PCIe 宽度（`pcie clock level` 中的 xN）              |
| `hcu_pcie_clock`                   | PCIe 时钟（对应 `pcie clock level`）                 |
| `hcu_pcie_replay_count`            | PCIe 重放计数（`PCIe Replay Count`）                 |
| `hcu_compute_unit_count`           | 计算单元总数                                         |
| `hcu_compute_unit_remaining_count` | 剩余可分配计算单元                                   |
| `hcu_memory_remaining`             | 剩余可分配显存                                       |
| `hcu_meminfo_used_bytes`           | Used Memory（标签 `mem_type`：vram/vis_vram/gtt）    |
| `hcu_meminfo_total_bytes`          | Total Memory（标签 `mem_type`）                      |
| `hcu_bad_pages`                    | Bad Page / 退役页数量（`--showpagesinfo`）           |
| `hcu_ce_count`                     | ECC 可纠正错误计数（标签 `block_type`，来自 `EccBlocksInfo`） |
| `hcu_ue_count`                     | ECC 不可纠正错误计数（标签 `block_type`，来自 `EccBlocksInfo`） |
| `hcu_ce_count_total`               | 全 block CE 聚合总量（`EccBlocksInfo` 求和）         |
| `hcu_ue_count_total`               | 全 block UE 聚合总量（`EccBlocksInfo` 求和）         |
| `hcu_ecc_enabled`                  | ECC 启用 block 位图                                  |
| `hcu_ras_block_enabled`            | RAS block 是否 ENABLED（`--showrasinfo`）            |
| `hcu_membank_ecc`                  | 显存 bank 级 ECC 数值字段（标签 `bank`/`field`）     |
| `hcu_df_bw_read`                   | DF 总线读带宽（中成本）                              |
| `hcu_df_bw_write`                  | DF 总线写带宽（中成本）                              |
| `hcu_df_bw_read_write`             | DF 总线读写总带宽（中成本）                          |
| `hcu_umc_bw_read`                  | UMC 读带宽汇总                                       |
| `hcu_umc_bw_write`                 | UMC 写带宽汇总                                       |
| `hcu_umc_bw_read_write`            | UMC 读写总带宽汇总                                   |
| `hcu_umc_chan_bw_read`             | UMC 分通道读带宽（标签 `chan_id`）                   |
| `hcu_umc_chan_bw_write`            | UMC 分通道写带宽                                     |
| `hcu_umc_chan_bw_read_write`       | UMC 分通道读写总带宽                                 |
| `hcu_hylink_send`                  | Hylink 发送带宽                                      |
| `hcu_hylink_recv`                  | Hylink 接收带宽                                      |
| `hcu_hylink_send_recv`             | Hylink 收发总带宽                                    |
| `hcu_xhcl_link_up`                 | XHCL/HSL 链路是否 UP（标签 `link_id`）               |
| `hcu_xhcl_link_state`              | XHCL/HSL 原始链路状态（标签 `link_id`）              |
| `hcu_xhcl_bw`                      | XHCL 带宽（标签 `link_id`/`direction`=recv|send）    |
| `hcu_xhcl_error_status`            | XHCL/HSL 错误状态码（ShowHSLErr）                    |
| `hcu_link_accessible`              | 链路可达性（对应 `Link accessible`）                 |
| `hcu_fan_level`                    | 风扇档位（fan level）                                |
| `hcu_fan_percent`                  | 风扇百分比（fan percentage）                         |
| `hcu_fan_rpm`                      | 风扇转速（RPM）                                      |
| `hcu_voltage_mv`                   | 电压（毫伏，经 `ShowVoltage`）                       |
| `hcu_encrypted_status`             | 加密状态（对应 `Encrypted status`）                  |
| `hcu_node_id`                      | Node ID（对应 `Node Id`）                            |
| `hcu_numa_node`                    | Numa Node                                            |
| `hcu_numa_affinity`                | Numa Affinity                                        |
| `hcu_health_status`                | 综合健康状态（`--healthcheck`；1=Healthy；标签 `health`） |
| `hcu_perf_level`                   | Performance Level（info，值恒为 1，标签 `level`）    |
| `hcu_process_vram_used_bytes`      | 进程显存占用（对应 `VRAM USED`，单位 bytes）         |
| `hcu_process_sdma_used`            | 进程 SDMA 占用（对应 `SDMA USED`）                   |
| `hcu_process_cu_occupancy`         | 进程 CU 占用，`ProcessHCUInfo`                       |
| `hcu_process_pasid`                | 进程 PASID                                           |
| `hcu_process_hcu_percent`          | 进程 HCU 占用率（对应 `HCU(%)`）                     |
| `hcu_process_vram_usage_rate`      | 进程显存占用率（%），`ProcessInfoByPid`              |
| `vhcu_count`                       | 物理卡上的 vHCU 数量                                 |

Hylink 指标在 `--hylink-detail=true` 时，会为每条链路额外输出带 `link_id` 标签的明细数据；否则 `link_id=all` 表示汇总值。

`hcu_throttle` 的 `throttle_type` 取值为 `thermal` / `power` / `slowdown` / `board_limit`。
`hcu_bad_pages` 的 `page_status` 含 `reserved` / `pending` / `unreservable` / `total`。

### vHCU 指标

| 内部指标名                | 说明            |
| ------------------------- | --------------- |
| `vhcu_temp`               | vHCU 温度       |
| `vhcu_sclk`               | vHCU 时钟频率   |
| `vhcu_utilizationrate`    | vHCU 利用率（`VDevBusyPercent`，整体 compute busy） |
| `vhcu_usedmemory_bytes`   | vHCU 已用显存   |
| `vhcu_usedmemory_percent` | vHCU 显存使用率 |

### 标签说明

**物理 HCU 通用标签：**

`device_id`, `minor_number`, `name`, `node`, `pcieBus_number`, `hcu_pod_namespace`, `hcu_pod_name`, `container`

**vHCU 额外标签：**

`vhcu_minor_number`, `vhcu_computer_unit`, `vhcu_memory_cap`

**Hylink / ECC / SE / 扩展标签：**

`link_id`（Hylink/XHCL）、`block_type`（ECC）、`se_id`（Shader Engine）、`sensor_type`（传感器温度）、`throttle_type`（降频类型）、`dst_minor_number`（P2P 目标设备）、`bank`/`field`（显存 bank ECC）、`level`（性能档位）、`pid`/`process_name`（进程指标）、`mem_type`（分类型显存）、`page_status`（保留页状态）、`chan_id`（UMC 通道）、`direction`（XHCL 带宽方向）、`health`（健康状态字符串）

以上均为默认 Label 名；可通过 `--label-define` 全局重命名。

### 指标输出示例

```text
# HELP hcu_temp hcu metrics of gauge
# TYPE hcu_temp gauge
hcu_temp{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="default",hcu_pod_name="training-job",container="worker"} 46

# HELP hcu_cu_usage HCU instantaneous CU usage rate (percent)
# TYPE hcu_cu_usage gauge
hcu_cu_usage{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="",hcu_pod_name="",container=""} 85

# HELP vhcu_utilizationrate vhcu metrics of gauge
# TYPE vhcu_utilizationrate gauge
vhcu_utilizationrate{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",hcu_pod_namespace="default",hcu_pod_name="inference",container="app",vhcu_minor_number="1",vhcu_computer_unit="8",vhcu_memory_cap="17179869184"} 72

# HELP hcu_process_vram_used_bytes Process VRAM USED in bytes
# TYPE hcu_process_vram_used_bytes gauge
hcu_process_vram_used_bytes{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="default",hcu_pod_name="training-job",container="worker",pid="12345",process_name="python"} 2147483648

# HELP hcu_process_sdma_used Process SDMA USED
# TYPE hcu_process_sdma_used gauge
hcu_process_sdma_used{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="default",hcu_pod_name="training-job",container="worker",pid="12345",process_name="python"} 1024

# HELP hcu_process_cu_occupancy Process CU occupancy from ProcessHCUInfo
# TYPE hcu_process_cu_occupancy gauge
hcu_process_cu_occupancy{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="default",hcu_pod_name="training-job",container="worker",pid="12345",process_name="python"} 64

# HELP hcu_process_pasid Process address space ID (PASID) from ProcessHCUInfo
# TYPE hcu_process_pasid gauge
hcu_process_pasid{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="default",hcu_pod_name="training-job",container="worker",pid="12345",process_name="python"} 7

# HELP hcu_process_hcu_percent Process HCU(%) usage
# TYPE hcu_process_hcu_percent gauge
hcu_process_hcu_percent{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="default",hcu_pod_name="training-job",container="worker",pid="12345",process_name="python"} 87.5

# HELP hcu_process_vram_usage_rate Process VRAM usage rate percent from ProcessInfoByPid
# TYPE hcu_process_vram_usage_rate gauge
hcu_process_vram_usage_rate{device_id="T8R1380013061601",minor_number="0",name="K100",node="hcunode3",pcieBus_number="0000:f6:00.0",hcu_pod_namespace="default",hcu_pod_name="training-job",container="worker",pid="12345",process_name="python"} 25
```

## Kubernetes Pod 关联

当 `--connect-k8s=true`（默认）且 Kubelet Pod Resources Socket 存在时，Exporter 会将 HCU 指标与 Pod / 容器关联，填充 `hcu_pod_namespace`、`hcu_pod_name`、`container` 标签。

### 工作机制

1. **Pod Informer**：监听本节点上申请了 `hygon.com/*` 资源的 Pod 增删改事件
2. **Pod Resources API**：通过 Unix Socket 查询设备 UUID 与 Pod / 容器的对应关系
3. **按需刷新**：仅在相关 Pod 发生变化时重新拉取设备映射，避免无效轮询

### 支持的资源类型

Exporter 根据 Pod 申请的资源名称，采用不同的关联策略：

| 场景                | 资源名前缀示例                                               | 关联方式                                                 |
| ------------------- | ------------------------------------------------------------ | -------------------------------------------------------- |
| 整卡 HCU            | `hygon.com/hcu`、`hygon.com/<型号>`、`hygon.com/<型号>_<显存>_<CU>` | 通过 PCIe Bus 号匹配 Pod Resources 中的设备 ID           |
| vHCU 共享           | `hygon.com/hcu-share`、`hygon.com/<前缀>-share-<CU>-<显存>`  | 通过 vHCU minor number 匹配                              |
| 动态 vHCU（hcunum） | `hygon.com/hcunum`、`hygon.com/<型号>_hcunum` 等             | 结合 `/etc/vdev/dynamic` 目录下的配置文件与 Pod UID 匹配 |

资源名列表会根据当前节点上检测到的 HCU 型号动态生成（见 `pkg/util/util.go` 中 `GetResourceNameList`）。

### 部署注意事项

- DaemonSet 需挂载 `/var/lib/kubelet`（只读）以访问 Pod Resources Socket
- 需挂载 `/etc/vdev` 以支持动态 vHCU 场景
- 需配置 RBAC 允许读取 Pod 信息（静态清单与 Helm Chart 均已包含）
- 裸机或非 K8s 环境请设置 `--connect-k8s=false`

## Prometheus 与 Grafana

### Prometheus 采集

**Kubernetes**：DaemonSet Pod 已携带 Prometheus 注解，集群内 Prometheus 可自动发现并抓取 `http://<node-ip>:16080/metrics`。

**裸机**：在 Prometheus 配置中手动添加 scrape job：

```yaml
scrape_configs:
  - job_name: hcu-exporter
    static_configs:
      - targets: ['<host>:16080']
    metrics_path: /metrics
```

若使用了 `--metrics-define` 重命名指标，PromQL 查询与告警规则需使用重命名后的指标名。

### Grafana Dashboard

`grafana/` 目录提供以下 Dashboard 模板：

| 文件                              | 说明                          |
| --------------------------------- | ----------------------------- |
| `hcu-exporter-dashboard.json`     | HCU Exporter 基础监控面板     |
| `hcu-exporter-k8s-dashboard.json` | 含 K8s Pod 关联信息的监控面板 |
| `hcu-cluster-monitor.json`        | 集群级 HCU 监控面板           |

在 Grafana 中通过 **Import → Upload JSON file** 导入即可。若启用了指标重命名，导入后需按实际指标名调整 Panel 查询。

## 项目结构

```
hcu-exporter/
├── cmd/
│   └── hcu-exporter/                 # 主程序入口（package main）
│       ├── main.go                  # CLI、指标定义、采集循环、HTTP 服务
│       └── metrics_extra.go         # 扩展指标采集与多标签指标写入
├── pkg/
│   ├── podresources/                # Kubelet Pod Resources 客户端与 vHCU 映射
│   │   ├── podresources.go
│   │   └── vhcupodinfo.go
│   └── util/                        # 资源名解析、指标/Label 自定义、K8s 工具函数
│       ├── util.go
│       └── metrics_define.go
├── deployment/
│   ├── static/hcu-exporter.yaml     # K8s 静态部署清单（DaemonSet + RBAC + Service）
│   ├── helm/hcu-exporter/           # Helm Chart
│   └── docker/hcu-exporter-start.sh # Docker 启动脚本
├── grafana/                         # Grafana Dashboard 模板
├── Dockerfile
├── build.sh                         # 编译、构建镜像、导出 tar
├── go.mod
└── LICENSE / NOTICE / THIRD_PARTY_NOTICES.md
```

## 常见问题

**Q: IDE 提示找不到 `github.com/HYGON-AI/hcu-dcgm/v3`？**

确保已 clone `hcu-dcgm` 到 `../hcu-dcgm`，并执行 `go mod tidy`。该模块通过 `replace` 使用本地路径，不会从 GitHub 远程拉取。

**Q: 编译报错 `drm/drm.h: No such file or directory`？**

请在 Linux 环境下编译，并安装 CGO 所需头文件（参考 hcu-dcgm 文档）。Windows 无法完成 CGO 编译。

**Q: 指标中没有 Pod 信息？**

确认 `--connect-k8s=true`（默认），Kubelet Pod Resources Socket 存在（`/var/lib/kubelet/pod-resources/kubelet.sock`），Pod 已申请 `hygon.com/*` 资源，且 RBAC 与 `/etc/vdev` 挂载配置正确。

**Q: 进程自动退出并提示 "Metrics Collect Stuck"？**

采集循环超过 60 秒未更新 `COLLECT_TIME` 时会主动退出，通常由 DCGM 调用阻塞引起。请检查驱动状态后重启 Exporter（K8s 下由 DaemonSet 自动拉起）。

**Q: `/metrics` 返回 403 Forbidden？**

检查是否配置了 `--ips` / `ALLOWED_IPS` 白名单，确认 Prometheus 采集节点的 IP 在允许范围内。

**Q: 启动时报 `metrics-define` 或 `label-define` 解析错误？**

- 确认 JSON 格式合法（建议使用单引号包裹整个参数，内部使用双引号）
- `--metrics-define` 的 Key 必须是有效的内部指标名；展示名需符合 Prometheus 命名规范且全局唯一
- `--label-define` 的 Key 为原 Label 名，Value 为展示 Label 名；展示名需符合 Prometheus 命名规范且全局唯一；重命名后同一指标内 Label 不能重名

**Q: `--enable-metrics` 和 `--metrics-define` 分别填什么名称？**

`--enable-metrics` 始终使用内部指标名（如 `hcu_ce_count`）；`--metrics-define` 的 Key 也是内部指标名，Value 为对外展示名。`--label-define` 使用原 Label 名作为 Key，对所有指标全局生效。

**Q: `--metrics-level` 与 `--enable-metrics` 如何选择？**

日常部署优先用 `--metrics-level`：多卡或短 scrape 间隔建议 `low`/`medium`；需要 PCIe 吞吐或采样利用率时再用 `high`。需要任意子集时用 `--enable-metrics`（会覆盖档位）。

## License

本项目名称：**hcu-exporter**。

本项目基于 [Apache License 2.0](LICENSE)（`Apache-2.0` / SPDX）开源。

Copyright (c) 2026 Hygon Information Technology Co., Ltd.

- 许可证全文：[LICENSE](LICENSE)
- 版权声明：[NOTICE](NOTICE)

## Third-Party

第三方依赖与来源清单见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。本仓库不内嵌 `vendor/` / `third_party/` 源码副本；Go 依赖通过 Modules 拉取，明细与许可证以该清单为准。

更多官方文档可参考 [光合开发者社区](https://cancon.hpccube.com:65024/1/main)。
