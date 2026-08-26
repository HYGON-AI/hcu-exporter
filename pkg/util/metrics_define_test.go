// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetricConfigMetricsDefine(t *testing.T) {
	cfg, err := NewMetricConfig(`{"hcu_ce_count":"ce_count","hcu_compute_unit_count":"compute_unit_count"}`, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.DisplayName("hcu_ce_count"); got != "ce_count" {
		t.Fatalf("DisplayName(hcu_ce_count) = %q, want ce_count", got)
	}
	if got := cfg.DisplayName("hcu_temp"); got != "hcu_temp" {
		t.Fatalf("DisplayName(hcu_temp) = %q, want hcu_temp", got)
	}
}

func TestNewMetricConfigDuplicateDisplayName(t *testing.T) {
	_, err := NewMetricConfig(`{"hcu_ce_count":"dup","hcu_ue_count":"dup"}`, "")
	if err == nil {
		t.Fatal("expected duplicate display name error")
	}
}

func TestNewMetricConfigDisplayNameConflictWithDefault(t *testing.T) {
	_, err := NewMetricConfig(`{"hcu_ce_count":"hcu_temp"}`, "")
	if err == nil {
		t.Fatal("expected conflict with default metric name")
	}
}

func TestNewMetricConfigLabelDefineGlobal(t *testing.T) {
	cfg, err := NewMetricConfig("", `{"block_type":"b_type","hcu_pod_name":"pod_name","device_id":"uuid"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ceLabels := cfg.LabelsForMetric("hcu_ce_count", HCUErrLabels)
	wantCE := []string{"uuid", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "pod_name", "container", "b_type"}
	for i := range wantCE {
		if ceLabels[i] != wantCE[i] {
			t.Fatalf("hcu_ce_count labels[%d] = %q, want %q (all=%v)", i, ceLabels[i], wantCE[i], ceLabels)
		}
	}

	dfLabels := cfg.LabelsForMetric("hcu_df_bw_read", HCULabels)
	if dfLabels[0] != "uuid" {
		t.Fatalf("hcu_df_bw_read first label = %q, want uuid", dfLabels[0])
	}

	remapped := cfg.RemapLabels(prometheus.Labels{
		"block_type":   "umc",
		"hcu_pod_name": "demo",
		"device_id":    "dev-1",
	})
	if remapped["b_type"] != "umc" || remapped["pod_name"] != "demo" || remapped["uuid"] != "dev-1" {
		t.Fatalf("unexpected remapped labels: %v", remapped)
	}
}

func TestNewMetricConfigLabelDefineDuplicateDisplayLabel(t *testing.T) {
	_, err := NewMetricConfig("", `{"device_id":"uuid","minor_number":"uuid"}`)
	if err == nil {
		t.Fatal("expected duplicate display label error")
	}
}

func TestNewMetricConfigLabelDefinePerMetricConflict(t *testing.T) {
	_, err := NewMetricConfig("", `{"device_id":"id","minor_number":"id"}`)
	if err == nil {
		t.Fatal("expected per-metric duplicate display label error")
	}
}
