#!/bin/bash
# Copyright (c) 2026 Hygon Information Technology Co., Ltd.
# SPDX-License-Identifier: Apache-2.0

mkdir -p /etc/vdev

# 访问 /metrics 的 IP 白名单（可选）：逗号分隔的 IP 或 CIDR，留空表示允许所有 IP
# 方式一：启动前 export ALLOWED_IPS="127.0.0.1,10.0.0.0/8"
# 方式二：在镜像名后追加启动参数，例如 -ips=127.0.0.1,10.0.0.0/8（见文件末尾注释）
ALLOWED_IPS="${ALLOWED_IPS:-}"

# 是否连接 K8S 采集 Pod 信息（可选）：true | false，默认 true
# 裸机（非 K8S 环境）部署时设置为 false
CONNECT_K8S="${CONNECT_K8S:-true}"

# 采集成本档位（可选）：low | medium | high，默认 high
# low    — 仅直接查询类指标（快，适合多卡 / 短采集间隔）
# medium — low + DF/Hylink/进程/健康检查
# high   — 全部指标（含约 1s 窗口类，如 PCIe 吞吐、采样利用率）
# 也可在镜像名后追加：--metrics-level=medium
METRICS_LEVEL="${METRICS_LEVEL:-high}"

# 采样类利用率窗口（可选，毫秒），默认 1000
# 影响 hcu_*_sampled_usage / hcu_cu_util / hcu_wave_util；也可追加 --sample-duration-ms=2000
SAMPLE_DURATION_MS="${SAMPLE_DURATION_MS:-1000}"

docker run --name hcu-exporter -d --privileged \
--device=/dev/kfd \
--device=/dev/mkfd \
--device=/dev/dri \
-v /etc/vdev:/etc/vdev \
-v /etc/hostname:/etc/hostname \
-v /opt/hyhal:/opt/hyhal \
-p 16080:16080 \
-e ALLOWED_IPS="${ALLOWED_IPS}" \
-e CONNECT_K8S="${CONNECT_K8S}" \
-e METRICS_LEVEL="${METRICS_LEVEL}" \
-e SAMPLE_DURATION_MS="${SAMPLE_DURATION_MS}" \
harbor.sourcefind.cn:5443/hcu/admin/base/hcu-exporter:v2.4.1

# ====== 示例 ======

# 1) 使用启动参数 -ips 限制访问来源：
# docker run --name hcu-exporter -d --privileged \
# --device=/dev/kfd --device=/dev/mkfd --device=/dev/dri \
# -v /etc/vdev:/etc/vdev -v /etc/hostname:/etc/hostname -v /opt/hyhal:/opt/hyhal \
# -p 16080:16080 \
# harbor.sourcefind.cn:5443/hcu/admin/base/hcu-exporter:v2.4.1 \
# -ips=127.0.0.1,10.0.0.0/8

# 2) 裸机部署，不连接 K8S：
# export CONNECT_K8S=false
# ./hcu-exporter-start.sh
#
# 或直接：
# docker run --name hcu-exporter -d --privileged \
# --device=/dev/kfd --device=/dev/mkfd --device=/dev/dri \
# -v /etc/vdev:/etc/vdev -v /etc/hostname:/etc/hostname -v /opt/hyhal:/opt/hyhal \
# -p 16080:16080 -e CONNECT_K8S=false \
# harbor.sourcefind.cn:5443/hcu/admin/base/hcu-exporter:v2.4.1

# 3) 按采集成本选用档位（环境变量）：
# export METRICS_LEVEL=medium
# ./hcu-exporter-start.sh
#
# 或直接：
# docker run --name hcu-exporter -d --privileged \
# --device=/dev/kfd --device=/dev/mkfd --device=/dev/dri \
# -v /etc/vdev:/etc/vdev -v /etc/hostname:/etc/hostname -v /opt/hyhal:/opt/hyhal \
# -p 16080:16080 -e METRICS_LEVEL=low -e CONNECT_K8S=false \
# harbor.sourcefind.cn:5443/hcu/admin/base/hcu-exporter:v2.4.1

# 4) 按采集成本选用档位（容器启动参数，等价于环境变量）：
# docker run --name hcu-exporter -d --privileged \
# --device=/dev/kfd --device=/dev/mkfd --device=/dev/dri \
# -v /etc/vdev:/etc/vdev -v /etc/hostname:/etc/hostname -v /opt/hyhal:/opt/hyhal \
# -p 16080:16080 -e CONNECT_K8S=false \
# harbor.sourcefind.cn:5443/hcu/admin/base/hcu-exporter:v2.4.1 \
# --metrics-level=medium
