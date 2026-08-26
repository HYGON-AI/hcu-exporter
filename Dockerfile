# Copyright (c) 2026 Hygon Information Technology Co., Ltd.
# SPDX-License-Identifier: Apache-2.0

FROM ubuntu:22.04

WORKDIR /root

RUN  apt update && apt install -y kmod pciutils

COPY LICENSE  /licenses/

COPY hcu-exporter /usr/local/bin/hcu-exporter

#COPY pkg/shim/lib ./lib

RUN chmod +x /usr/local/bin/hcu-exporter
#    && ln -s /root/lib/librocm_smi64.so.2.8 /root/lib/librocm_smi64.so.2 \
#    && ln -s /root/lib/librocm_smi64.so.2 /root/lib/librocm_smi64.so

ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/opt/hyhal/lib:/root/lib

EXPOSE 16080

CMD  ["/usr/local/bin/hcu-exporter"]
