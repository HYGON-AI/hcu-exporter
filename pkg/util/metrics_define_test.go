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

func TestResourcePrefixRewritesExportedNames(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})
	if err := InitResourcePrefix("dcu"); err != nil {
		t.Fatalf("init prefix: %v", err)
	}

	cfg, err := NewMetricConfig("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.FamilyToken() != "dcu" {
		t.Fatalf("FamilyToken() = %q, want dcu", cfg.FamilyToken())
	}
	if got := cfg.DisplayName("hcu_ce_count"); got != "dcu_ce_count" {
		t.Fatalf("DisplayName(hcu_ce_count) = %q, want dcu_ce_count", got)
	}
	if got := cfg.DisplayName("vhcu_utilizationrate"); got != "vdcu_utilizationrate" {
		t.Fatalf("DisplayName(vhcu_utilizationrate) = %q, want vdcu_utilizationrate", got)
	}
	if got := cfg.DisplayName("hcu_process_hcu_percent"); got != "dcu_process_hcu_percent" {
		t.Fatalf("DisplayName(hcu_process_hcu_percent) = %q, want dcu_process_hcu_percent", got)
	}

	labels := cfg.LabelsForMetric("vhcu_utilizationrate", VHcuLabels)
	want := []string{
		"vdcu_minor_number", "vdcu_computer_unit", "vdcu_memory_cap",
		"device_id", "minor_number", "name", "node",
		"dcu_pod_namespace", "dcu_pod_name", "container",
	}
	if len(labels) != len(want) {
		t.Fatalf("labels = %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("labels[%d] = %q, want %q (all=%v)", i, labels[i], want[i], labels)
		}
	}

	remapped := cfg.RemapLabels(prometheus.Labels{
		"hcu_pod_name":      "demo",
		"vhcu_minor_number": "1",
		"device_id":         "dev-1",
	})
	if remapped["dcu_pod_name"] != "demo" || remapped["vdcu_minor_number"] != "1" || remapped["device_id"] != "dev-1" {
		t.Fatalf("unexpected remapped labels: %v", remapped)
	}
}

func TestMetricsAndLabelDefineOverrideResourcePrefix(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})
	if err := InitResourcePrefix("dcu"); err != nil {
		t.Fatalf("init prefix: %v", err)
	}

	cfg, err := NewMetricConfig(
		`{"dcu_ce_count":"ce_count","dcu_temp":"hcu_temp"}`,
		`{"dcu_pod_name":"pod_name","vdcu_minor_number":"vhcu_minor_number"}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.DisplayName("hcu_ce_count"); got != "ce_count" {
		t.Fatalf("DisplayName(hcu_ce_count) = %q, want ce_count", got)
	}
	if got := cfg.DisplayName("hcu_temp"); got != "hcu_temp" {
		t.Fatalf("DisplayName(hcu_temp) = %q, want hcu_temp", got)
	}
	if got := cfg.DisplayName("vhcu_utilizationrate"); got != "vdcu_utilizationrate" {
		t.Fatalf("DisplayName(vhcu_utilizationrate) = %q, want vdcu_utilizationrate", got)
	}

	labels := cfg.LabelsForMetric("vhcu_temp", VHcuLabels)
	got := map[string]struct{}{}
	for _, label := range labels {
		got[label] = struct{}{}
	}
	for _, label := range []string{"pod_name", "vhcu_minor_number", "dcu_pod_namespace", "vdcu_computer_unit", "vdcu_memory_cap"} {
		if _, ok := got[label]; !ok {
			t.Fatalf("missing label %q in %v", label, labels)
		}
	}
	if _, ok := got["dcu_pod_name"]; ok {
		t.Fatalf("LABEL_DEFINE should keep hcu_pod_name from becoming dcu_pod_name: %v", labels)
	}
}

func TestResourcePrefixMetricNameConflict(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})
	if err := InitResourcePrefix("dcu"); err != nil {
		t.Fatalf("init prefix: %v", err)
	}
	_, err := NewMetricConfig(`{"dcu_ce_count":"dcu_temp"}`, "")
	if err == nil {
		t.Fatal("expected display name conflict with resource-prefix rewrite of hcu_temp")
	}
}

func TestDefineKeysUsePrefixedNames(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})
	if err := InitResourcePrefix("dcu"); err != nil {
		t.Fatalf("init prefix: %v", err)
	}

	cfg, err := NewMetricConfig(`{"dcu_ce_count":"ce_count"}`, `{"dcu_pod_name":"pod_name","block_type":"b_type"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.DisplayName("hcu_ce_count"); got != "ce_count" {
		t.Fatalf("DisplayName(hcu_ce_count) = %q, want ce_count", got)
	}
	if got := cfg.DisplayName("hcu_temp"); got != "dcu_temp" {
		t.Fatalf("DisplayName(hcu_temp) = %q, want dcu_temp", got)
	}
	ceLabels := cfg.LabelsForMetric("hcu_ce_count", HCUErrLabels)
	hasPod, hasBlock := false, false
	for _, label := range ceLabels {
		if label == "pod_name" {
			hasPod = true
		}
		if label == "b_type" {
			hasBlock = true
		}
		if label == "dcu_pod_name" || label == "hcu_pod_name" || label == "block_type" {
			t.Fatalf("unexpected label %q in %v", label, ceLabels)
		}
	}
	if !hasPod || !hasBlock {
		t.Fatalf("labels = %v, want pod_name and b_type", ceLabels)
	}

	if _, err := NewMetricConfig(`{"hcu_ce_count":"ce_count"}`, ""); err == nil {
		t.Fatal("expected unknown metric when METRICS_DEFINE key is the built-in name under prefix dcu")
	}
	if _, err := NewMetricConfig("", `{"hcu_pod_name":"pod_name"}`); err == nil {
		t.Fatal("expected unknown label when LABEL_DEFINE key is the built-in name under prefix dcu")
	}
}

func TestStarResourcePrefixSanitizesMetricFamily(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})
	if err := InitResourcePrefix("*cu"); err != nil {
		t.Fatalf("init prefix: %v", err)
	}
	cfg, err := NewMetricConfig("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.FamilyToken() != "cu" {
		t.Fatalf("FamilyToken() = %q, want cu", cfg.FamilyToken())
	}
	if got := cfg.DisplayName("hcu_ce_count"); got != "cu_ce_count" {
		t.Fatalf("DisplayName(hcu_ce_count) = %q, want cu_ce_count", got)
	}
	if got := cfg.DisplayName("vhcu_utilizationrate"); got != "vcu_utilizationrate" {
		t.Fatalf("DisplayName(vhcu_utilizationrate) = %q, want vcu_utilizationrate", got)
	}

	defined, err := NewMetricConfig(`{"cu_ce_count":"ce_count"}`, `{"cu_pod_name":"pod_name"}`)
	if err != nil {
		t.Fatalf("prefixed key: %v", err)
	}
	if got := defined.DisplayName("hcu_ce_count"); got != "ce_count" {
		t.Fatalf("DisplayName(hcu_ce_count) = %q, want ce_count", got)
	}
	if _, err := NewMetricConfig(`{"*cu_ce_count":"ce_count"}`, ""); err == nil {
		t.Fatal("expected unknown metric for unsanitized *cu key")
	}
}

func TestInvalidResourcePrefixMetricFamily(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})
	if err := InitResourcePrefix("9cu"); err != nil {
		t.Fatalf("init prefix: %v", err)
	}
	if _, err := NewMetricConfig("", ""); err == nil {
		t.Fatal("expected error for metric family starting with a digit")
	}
}
