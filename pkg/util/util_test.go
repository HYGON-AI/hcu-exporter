// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestResolveEnableMetrics(t *testing.T) {
	got := ResolveEnableMetrics("")
	if len(got) != len(allInternalMetricNames) {
		t.Fatalf("ResolveEnableMetrics empty: got %d metrics, want %d", len(got), len(allInternalMetricNames))
	}

	got = ResolveEnableMetrics("hcu_temp,vhcu_temp")
	if len(got) != 2 || got[0] != "hcu_temp" || got[1] != "vhcu_temp" {
		t.Fatalf("ResolveEnableMetrics valid names: got %v", got)
	}

	got = ResolveEnableMetrics("unknown_metric")
	if len(got) != 0 {
		t.Fatalf("ResolveEnableMetrics invalid name should be skipped: got %v", got)
	}
}

func TestMetricsByLevel(t *testing.T) {
	low, err := MetricsByLevel(MetricsLevelLow)
	if err != nil {
		t.Fatalf("low: %v", err)
	}
	if len(low) != len(metricsLevelLow) {
		t.Fatalf("low len=%d, want %d", len(low), len(metricsLevelLow))
	}

	medium, err := MetricsByLevel(MetricsLevelMedium)
	if err != nil {
		t.Fatalf("medium: %v", err)
	}
	wantMedium := len(metricsLevelLow) + len(metricsLevelMediumExtra)
	if len(medium) != wantMedium {
		t.Fatalf("medium len=%d, want %d", len(medium), wantMedium)
	}

	high, err := MetricsByLevel(MetricsLevelHigh)
	if err != nil {
		t.Fatalf("high: %v", err)
	}
	if len(high) != len(allInternalMetricNames) {
		t.Fatalf("high len=%d, want %d", len(high), len(allInternalMetricNames))
	}

	if _, err := MetricsByLevel("invalid"); err == nil {
		t.Fatal("expected error for invalid metrics-level")
	}

	// Ensure high-cost metrics are absent from low/medium.
	for _, name := range metricsLevelHighExtra {
		if containsMetric(low, name) {
			t.Fatalf("low should not include high-cost metric %s", name)
		}
		if containsMetric(medium, name) {
			t.Fatalf("medium should not include high-cost metric %s", name)
		}
		if !containsMetric(high, name) {
			t.Fatalf("high should include high-cost metric %s", name)
		}
	}
}

func TestResolveEnabledMetrics(t *testing.T) {
	got, err := ResolveEnabledMetrics(MetricsLevelLow, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(metricsLevelLow) {
		t.Fatalf("level low: got %d, want %d", len(got), len(metricsLevelLow))
	}

	got, err = ResolveEnabledMetrics(MetricsLevelLow, "hcu_temp,hcu_pciebw_mb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "hcu_temp" || got[1] != "hcu_pciebw_mb" {
		t.Fatalf("enable-metrics should override level: got %v", got)
	}
}

func TestMetricLevelCoverageMatchesAll(t *testing.T) {
	seen := make(map[string]struct{}, len(allInternalMetricNames))
	for _, name := range metricsLevelLow {
		if _, ok := seen[name]; ok {
			t.Fatalf("duplicate metric in low: %s", name)
		}
		seen[name] = struct{}{}
	}
	for _, name := range metricsLevelMediumExtra {
		if _, ok := seen[name]; ok {
			t.Fatalf("duplicate metric across levels: %s", name)
		}
		seen[name] = struct{}{}
	}
	for _, name := range metricsLevelHighExtra {
		if _, ok := seen[name]; ok {
			t.Fatalf("duplicate metric across levels: %s", name)
		}
		seen[name] = struct{}{}
	}
	if len(seen) != len(allInternalMetricNames) {
		t.Fatalf("level union size %d != allInternalMetricNames %d", len(seen), len(allInternalMetricNames))
	}
	for _, name := range allInternalMetricNames {
		if _, ok := seen[name]; !ok {
			t.Fatalf("metric %s missing from level presets", name)
		}
	}
}

func containsMetric(list []string, name string) bool {
	for _, item := range list {
		if item == name {
			return true
		}
	}
	return false
}

func TestAllPodContainers(t *testing.T) {
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{{Name: "init"}},
			Containers:     []v1.Container{{Name: "app"}},
		},
	}
	got := AllPodContainers(pod)
	if len(got) != 2 || got[0].Name != "init" || got[1].Name != "app" {
		t.Fatalf("AllPodContainers: got %#v", got)
	}
	if AllPodContainers(nil) != nil {
		t.Fatal("AllPodContainers(nil) should be nil")
	}
}

func TestRequestsHCUIncludesInitContainers(t *testing.T) {
	qty := resource.MustParse("1")
	initOnly := &v1.Pod{
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{{
				Name: "init-hcu",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName(WholeCardResourceName()): qty,
					},
				},
			}},
			Containers: []v1.Container{{Name: "app"}},
		},
	}
	if !RequestsHCU(initOnly) {
		t.Fatal("RequestsHCU should be true when only init container requests HCU")
	}

	appOnly := &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name: "app",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName(WholeCardResourceName()): qty,
					},
				},
			}},
		},
	}
	if !RequestsHCU(appOnly) {
		t.Fatal("RequestsHCU should be true when app container requests HCU")
	}

	none := &v1.Pod{Spec: v1.PodSpec{Containers: []v1.Container{{Name: "app"}}}}
	if RequestsHCU(none) {
		t.Fatal("RequestsHCU should be false when no container requests HCU")
	}
}

func TestGetHCUResourceNamesIncludesInitContainers(t *testing.T) {
	qty := resource.MustParse("1")
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{{
				Name: "init-hcu",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName(WholeCardResourceName()): qty,
					},
				},
			}},
			Containers: []v1.Container{{
				Name: "app",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName(DynamicNumResourceName()): qty,
					},
				},
			}},
		},
	}
	names := GetHCUResourceNames(pod)
	if !names.Contains(WholeCardResourceName()) || !names.Contains(DynamicNumResourceName()) {
		t.Fatalf("GetHCUResourceNames missing init/app resources: %v", names.ToSlice())
	}
}
