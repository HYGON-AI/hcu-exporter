// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HCULabels        = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container"}
	VHcuLabels       = []string{"vhcu_minor_number", "vhcu_computer_unit", "vhcu_memory_cap", "device_id", "minor_number", "name", "node", "hcu_pod_namespace", "hcu_pod_name", "container"}
	HCUErrLabels     = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "block_type"}
	HylinkLabels     = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "link_id"}
	SELabels         = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "se_id"}
	SensorLabels     = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "sensor_type"}
	ThrottleLabels   = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "throttle_type"}
	P2PLabels        = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "dst_minor_number"}
	MemBankLabels    = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "bank", "field"}
	PerfLabels       = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "level"}
	ProcessLabels    = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "pid", "process_name"}
	MemTypeLabels    = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "mem_type"}
	PageStatusLabels = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "page_status"}
	ChanLabels       = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "chan_id"}
	XhclBwLabels     = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "link_id", "direction"}
	HealthLabels     = []string{"device_id", "minor_number", "name", "node", "pcieBus_number", "hcu_pod_namespace", "hcu_pod_name", "container", "health"}
)

var metricBaseLabels = map[string][]string{
	"hcu_temp":                         HCULabels,
	"hcu_power_usage":                  HCULabels,
	"hcu_powercap":                     HCULabels,
	"hcu_sclk":                         HCULabels,
	"hcu_mclk":                         HCULabels,
	"hcu_utilizationrate":              HCULabels,
	"hcu_usedmemory_bytes":             HCULabels,
	"hcu_memorycap_bytes":              HCULabels,
	"hcu_pciebw_mb":                    HCULabels,
	"hcu_pcie_sent_mb":                 HCULabels,
	"hcu_pcie_receive_mb":              HCULabels,
	"hcu_compute_unit_count":           HCULabels,
	"hcu_compute_unit_remaining_count": HCULabels,
	"hcu_memory_remaining":             HCULabels,
	"hcu_ce_count":                     HCUErrLabels,
	"hcu_ue_count":                     HCUErrLabels,
	"hcu_ce_count_total":               HCULabels,
	"hcu_ue_count_total":               HCULabels,
	"hcu_df_bw_read":                   HCULabels,
	"hcu_df_bw_write":                  HCULabels,
	"hcu_df_bw_read_write":             HCULabels,
	"hcu_hylink_send":                  HylinkLabels,
	"hcu_hylink_recv":                  HylinkLabels,
	"hcu_hylink_send_recv":             HylinkLabels,
	"hcu_cu_usage":                     HCULabels,
	"hcu_sampled_usage":                HCULabels,
	"hcu_cu_sampled_usage":             HCULabels,
	"hcu_wave_sampled_usage":           HCULabels,
	"hcu_se_usage":                     SELabels,
	"hcu_temp_mem":                     HCULabels,
	"hcu_temp_board":                   HCULabels,
	"hcu_sensor_temp":                  SensorLabels,
	"hcu_throttle":                     ThrottleLabels,
	"hcu_cu_util":                      HCULabels,
	"hcu_wave_util":                    HCULabels,
	"hcu_sclk_max":                     HCULabels,
	"hcu_mclk_max":                     HCULabels,
	"hcu_xhcl_link_up":                 HylinkLabels,
	"hcu_xhcl_link_state":              HylinkLabels,
	"hcu_available_memory_bytes":       HCULabels,
	"hcu_link_accessible":              P2PLabels,
	"hcu_memory_overdrive":             HCULabels,
	"hcu_membank_ecc":                  MemBankLabels,
	"hcu_fan_level":                    HCULabels,
	"hcu_fan_percent":                  HCULabels,
	"hcu_fan_rpm":                      HCULabels,
	"hcu_vram_percent":                 HCULabels,
	"hcu_util_percent":                 HCULabels,
	"hcu_umc_bw_read":                  HCULabels,
	"hcu_umc_bw_write":                 HCULabels,
	"hcu_umc_bw_read_write":            HCULabels,
	"hcu_pcie_replay_count":            HCULabels,
	"hcu_pcie_width":                   HCULabels,
	"hcu_pcie_clock":                   HCULabels,
	"hcu_xhcl_error_status":            HCULabels,
	"hcu_perf_level":                   PerfLabels,
	"hcu_process_vram_used_bytes":      ProcessLabels,
	"hcu_process_sdma_used":            ProcessLabels,
	"hcu_process_cu_occupancy":         ProcessLabels,
	"hcu_process_pasid":                ProcessLabels,
	"hcu_process_hcu_percent":          ProcessLabels,
	"hcu_process_vram_usage_rate":      ProcessLabels,
	"hcu_temp_edge_max":                HCULabels,
	"hcu_temp_edge_critical":           HCULabels,
	"hcu_sensor_temp_max":              SensorLabels,
	"hcu_sensor_temp_critical":         SensorLabels,
	"hcu_overdrive":                    HCULabels,
	"hcu_powercap_range_max":           HCULabels,
	"hcu_powercap_range_min":           HCULabels,
	"hcu_meminfo_used_bytes":           MemTypeLabels,
	"hcu_meminfo_total_bytes":          MemTypeLabels,
	"hcu_bad_pages":                    PageStatusLabels,
	"hcu_ecc_enabled":                  HCULabels,
	"hcu_ras_block_enabled":            HCUErrLabels,
	"hcu_xhcl_bw":                      XhclBwLabels,
	"hcu_umc_chan_bw_read":             ChanLabels,
	"hcu_umc_chan_bw_write":            ChanLabels,
	"hcu_umc_chan_bw_read_write":       ChanLabels,
	"hcu_voltage_mv":                   HCULabels,
	"hcu_encrypted_status":             HCULabels,
	"hcu_node_id":                      HCULabels,
	"hcu_numa_node":                    HCULabels,
	"hcu_numa_affinity":                HCULabels,
	"hcu_health_status":                HealthLabels,
	"vhcu_count":                       HCULabels,
	"vhcu_temp":                        VHcuLabels,
	"vhcu_sclk":                        VHcuLabels,
	"vhcu_utilizationrate":             VHcuLabels,
	"vhcu_usedmemory_bytes":            VHcuLabels,
	"vhcu_usedmemory_percent":          VHcuLabels,
}

var promNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// MetricConfig holds metric and label display name mappings.
type MetricConfig struct {
	displayNames map[string]string
	labelMap     map[string]string
}

func (c *MetricConfig) DisplayName(internal string) string {
	if c == nil {
		return internal
	}
	if name, ok := c.displayNames[internal]; ok && name != "" {
		return name
	}
	return internal
}

func (c *MetricConfig) LabelsForMetric(_ string, baseLabels []string) []string {
	return applyLabelMapping(baseLabels, c.labelMap)
}

func (c *MetricConfig) RemapLabels(labels prometheus.Labels) prometheus.Labels {
	if c == nil || len(c.labelMap) == 0 {
		return labels
	}
	result := make(prometheus.Labels, len(labels))
	for key, value := range labels {
		if newKey, ok := c.labelMap[key]; ok {
			result[newKey] = value
		} else {
			result[key] = value
		}
	}
	return result
}

func applyLabelMapping(baseLabels []string, mapping map[string]string) []string {
	if len(mapping) == 0 {
		return append([]string(nil), baseLabels...)
	}
	result := make([]string, len(baseLabels))
	for i, label := range baseLabels {
		if newLabel, ok := mapping[label]; ok {
			result[i] = newLabel
		} else {
			result[i] = label
		}
	}
	return result
}

func validateGlobalLabelMap(labelMap map[string]string) error {
	if len(labelMap) == 0 {
		return nil
	}

	usedDisplayLabels := make(map[string]string, len(labelMap))
	for original, display := range labelMap {
		original = strings.TrimSpace(original)
		display = strings.TrimSpace(display)
		if original == "" {
			return fmt.Errorf("label-define: original label name must not be empty")
		}
		if err := validatePromName(display, "label-define display label"); err != nil {
			return err
		}
		if existing, ok := usedDisplayLabels[display]; ok {
			return fmt.Errorf("label-define: duplicate display label %q for %q and %q", display, existing, original)
		}
		usedDisplayLabels[display] = original
	}

	for internal, baseLabels := range metricBaseLabels {
		displayLabels := applyLabelMapping(baseLabels, labelMap)
		seen := make(map[string]struct{}, len(displayLabels))
		for _, label := range displayLabels {
			if _, ok := seen[label]; ok {
				return fmt.Errorf("label-define: metric %q has duplicate display label %q after remapping", internal, label)
			}
			seen[label] = struct{}{}
		}
	}
	return nil
}

func validatePromName(name, field string) error {
	if name == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if !promNamePattern.MatchString(name) {
		return fmt.Errorf("%s %q is not a valid Prometheus name", field, name)
	}
	return nil
}

// NewMetricConfig parses --metrics-define and --label-define JSON strings.
func NewMetricConfig(metricsDefineJSON, labelDefineJSON string) (*MetricConfig, error) {
	cfg := &MetricConfig{
		displayNames: make(map[string]string),
		labelMap:     make(map[string]string),
	}
	if err := cfg.applyMetricsDefine(metricsDefineJSON); err != nil {
		return nil, err
	}
	if err := cfg.applyLabelDefine(labelDefineJSON); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *MetricConfig) applyMetricsDefine(metricsDefineJSON string) error {
	metricsDefineJSON = strings.TrimSpace(metricsDefineJSON)
	if metricsDefineJSON == "" {
		return nil
	}
	raw := make(map[string]string)
	if err := json.Unmarshal([]byte(metricsDefineJSON), &raw); err != nil {
		return fmt.Errorf("parse metrics-define: %w", err)
	}
	usedDisplayNames := make(map[string]string)
	for internal, display := range raw {
		if !isValidInternalMetricName(internal) {
			return fmt.Errorf("metrics-define: unknown metric %q", internal)
		}
		display = strings.TrimSpace(display)
		if err := validatePromName(display, "metrics-define display name"); err != nil {
			return err
		}
		if existing, ok := usedDisplayNames[display]; ok {
			return fmt.Errorf("metrics-define: duplicate display name %q for %q and %q", display, existing, internal)
		}
		usedDisplayNames[display] = internal
		c.displayNames[internal] = display
	}

	for _, internal := range allInternalMetricNames {
		display := c.DisplayName(internal)
		if owner, ok := usedDisplayNames[display]; ok && owner != internal {
			return fmt.Errorf("metrics-define: display name %q conflicts between %q and %q", display, owner, internal)
		}
		if _, mapped := c.displayNames[internal]; !mapped {
			if _, exists := usedDisplayNames[display]; exists {
				return fmt.Errorf("metrics-define: display name %q conflicts with renamed metric %q", display, usedDisplayNames[display])
			}
			usedDisplayNames[display] = internal
		}
	}
	return nil
}

func (c *MetricConfig) applyLabelDefine(labelDefineJSON string) error {
	labelDefineJSON = strings.TrimSpace(labelDefineJSON)
	if labelDefineJSON == "" {
		return nil
	}
	raw := make(map[string]string)
	if err := json.Unmarshal([]byte(labelDefineJSON), &raw); err != nil {
		return fmt.Errorf("parse label-define: %w", err)
	}

	labelMap := make(map[string]string, len(raw))
	for original, display := range raw {
		original = strings.TrimSpace(original)
		display = strings.TrimSpace(display)
		if prev, ok := labelMap[original]; ok && prev != display {
			return fmt.Errorf("label-define: label %q maps to multiple names", original)
		}
		labelMap[original] = display
	}
	if err := validateGlobalLabelMap(labelMap); err != nil {
		return err
	}
	c.labelMap = labelMap
	return nil
}

func BaseLabelsForMetric(internal string) []string {
	labels, ok := metricBaseLabels[internal]
	if !ok {
		return nil
	}
	return append([]string(nil), labels...)
}
