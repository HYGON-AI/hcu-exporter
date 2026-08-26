# Copyright (c) 2026 Hygon Information Technology Co., Ltd.
# SPDX-License-Identifier: Apache-2.0

apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: hcu-exporter
  labels:
  {{- include "hcu-exporter.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: hcu-exporter
      app.kubernetes.io/version: {{ .Values.hcuExporter.hcuExporter.image.tag }}
    {{- include "hcu-exporter.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: hcu-exporter
        app.kubernetes.io/version: {{ .Values.hcuExporter.hcuExporter.image.tag }}
      {{- include "hcu-exporter.selectorLabels" . | nindent 8 }}
      annotations:
        prometheus.io/path: metrics
        prometheus.io/port: "16080"
        prometheus.io/scrape: "true"
    spec:
      containers:
      - env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: HCU_EXPORTER_LISTEN
          value: {{ quote .Values.hcuExporter.hcuExporter.env.hcuExporterListen }}
        - name: PULSE
          value: {{ quote .Values.hcuExporter.hcuExporter.env.pulse }}
        - name: METRICS_LEVEL
          value: {{ quote .Values.hcuExporter.hcuExporter.env.metricsLevel }}
        - name: SAMPLE_DURATION_MS
          value: {{ quote .Values.hcuExporter.hcuExporter.env.sampleDurationMs }}
        {{- if .Values.hcuExporter.hcuExporter.env.enableMetrics }}
        - name: ENABLE_METRICS
          value: {{ quote .Values.hcuExporter.hcuExporter.env.enableMetrics }}
        {{- end }}
        - name: LOG_THRESHOLD
          value: {{ quote .Values.hcuExporter.hcuExporter.env.logThreshold }}
        - name: LOG_VERBOSE
          value: {{ quote .Values.hcuExporter.hcuExporter.env.logVerbose }}
        - name: LOG_OUTPUT
          value: {{ quote .Values.hcuExporter.hcuExporter.env.logOutput }}
        - name: ALLOWED_IPS
          value: {{ quote .Values.hcuExporter.hcuExporter.env.allowedIPs }}
        - name: CONNECT_K8S
          value: {{ quote .Values.hcuExporter.hcuExporter.env.connectK8s }}
        - name: KUBERNETES_CLUSTER_DOMAIN
          value: {{ quote .Values.kubernetesClusterDomain }}
        image: {{ .Values.hcuExporter.hcuExporter.image.repository }}:{{ .Values.hcuExporter.hcuExporter.image.tag
          | default .Chart.AppVersion }}
        imagePullPolicy: {{ .Values.hcuExporter.hcuExporter.imagePullPolicy }}
        name: hcu-exporter
        ports:
        - containerPort: 16080
          hostPort: 16080
          name: metrics
        resources: {}
        securityContext: {{- toYaml .Values.hcuExporter.hcuExporter.containerSecurityContext
          | nindent 10 }}
        volumeMounts:
        {{- if eq .Values.hcuExporter.hcuExporter.env.connectK8s "true" }}
        - mountPath: /var/lib/kubelet
          name: kubelet
          readOnly: true
        {{- end }}
        - mountPath: /dev
          name: dev
        - mountPath: /etc/hostname
          name: hostname
          readOnly: true
        - mountPath: /etc/vdev
          name: vdev
          readOnly: true
        - mountPath: /opt/hyhal
          name: hyhal
          readOnly: true
      hostNetwork: true
      nodeSelector: {{- toYaml .Values.hcuExporter.nodeSelector | nindent 8 }}
      {{- if eq .Values.hcuExporter.hcuExporter.env.connectK8s "true" }}
      serviceAccount: pod-reader
      {{- end }}
      volumes:
      {{- if eq .Values.hcuExporter.hcuExporter.env.connectK8s "true" }}
      - hostPath:
          path: /var/lib/kubelet
        name: kubelet
      {{- end }}
      - hostPath:
          path: /dev
        name: dev
      - hostPath:
          path: /etc/hostname
        name: hostname
      - hostPath:
          path: /etc/vdev
          type: DirectoryOrCreate
        name: vdev
      - hostPath:
          path: /opt/hyhal
        name: hyhal
