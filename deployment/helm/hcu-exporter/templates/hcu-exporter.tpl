# Copyright (c) 2026 Hygon Information Technology Co., Ltd.
# SPDX-License-Identifier: Apache-2.0

apiVersion: v1
kind: Service
metadata:
  name: hcu-exporter
  labels:
  {{- include "hcu-exporter.labels" . | nindent 4 }}
spec:
  type: {{ .Values.hcuExporter.type }}
  selector:
    app.kubernetes.io/name: hcu-exporter
    app.kubernetes.io/version: {{ .Values.hcuExporter.hcuExporter.image.tag }}
  {{- include "hcu-exporter.selectorLabels" . | nindent 4 }}
  ports:
    {{- .Values.hcuExporter.ports | toYaml | nindent 2 -}}
