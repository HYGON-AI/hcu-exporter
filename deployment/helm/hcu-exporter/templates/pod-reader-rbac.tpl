# Copyright (c) 2026 Hygon Information Technology Co., Ltd.
# SPDX-License-Identifier: Apache-2.0

{{- if eq .Values.hcuExporter.hcuExporter.env.connectK8s "true" }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pod-reader
  labels:
  {{- include "hcu-exporter.labels" . | nindent 4 }}
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - '*'
- apiGroups:
  - storage.k8s.io
  resources:
  - pods
  verbs:
  - '*'
{{- end }}
