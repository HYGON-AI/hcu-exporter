# Copyright (c) 2026 Hygon Information Technology Co., Ltd.
# SPDX-License-Identifier: Apache-2.0

{{- if eq .Values.hcuExporter.hcuExporter.env.connectK8s "true" }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pod-reader
  labels:
  {{- include "hcu-exporter.labels" . | nindent 4 }}
{{- end }}
