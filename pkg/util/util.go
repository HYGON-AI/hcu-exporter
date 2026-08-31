// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/HYGON-AI/hcu-dcgm/v3/pkg/dcgm"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
)

const RESOURCE_PREFIX = "hygon.com"

func ParseStringToArray(input string, comma string) []string {
	parts := strings.Split(input, comma)
	result := make([]string, 0, len(parts))

	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func StringSliceToSet(slice []string) mapset.Set[string] {
	set := mapset.NewSet[string]()
	set.Append(slice...) // Append adds all elements in batch
	return set
}

func HasIntersection[T comparable](a, b mapset.Set[T]) bool {
	return a.Intersect(b).Cardinality() > 0
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func EnsureDirExists(path string) error {
	if !FileExists(path) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		return nil
	}
	return nil
}

func RequestsHCU(pod *v1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		for resourceName, quantity := range container.Resources.Limits {
			if (strings.Contains(string(resourceName), RESOURCE_PREFIX)) && quantity.Value() > 0 {
				return true
			}
		}
	}
	return false
}

func GetHCUResourceNames(pod *v1.Pod) mapset.Set[string] {
	resourceNames := mapset.NewSet[string]()
	for _, container := range pod.Spec.Containers {
		for resourceName, quantity := range container.Resources.Limits {
			if (strings.Contains(string(resourceName), RESOURCE_PREFIX)) && quantity.Value() > 0 {
				resourceNames.Add(string(resourceName))
			}
		}
	}
	return resourceNames
}

func GetResourceNameList(vHCUFlag bool) []string {
	info, err := dcgm.GetDeviceInfo(0)
	if err != nil {
		glog.Errorf("Get device info err: %v", err)
	}
	cu_string := strconv.Itoa(info.ComputeUnitCount) + "CU"
	mem_string := strconv.Itoa(int(math.Round(float64(info.GlobalMemSize)/(1024*1024*1024)))) + "G"

	if vHCUFlag {
		return []string{RESOURCE_PREFIX + "/hcunum", RESOURCE_PREFIX + "/" + info.Name + "_hcunum", RESOURCE_PREFIX + "/" + strings.ReplaceAll(info.Name, "_", "-") + "_" + mem_string + "_" + cu_string + "_hcunum"}
	}

	return []string{RESOURCE_PREFIX + "/hcu", RESOURCE_PREFIX + "/" + info.Name, RESOURCE_PREFIX + "/" + strings.ReplaceAll(info.Name, "_", "-") + "_" + mem_string + "_" + cu_string}
}

// Metrics collection cost levels. Tiers are cumulative: medium ⊇ low, high ⊇ medium.
const (
	MetricsLevelLow    = "low"
	MetricsLevelMedium = "medium"
	MetricsLevelHigh   = "high"
)

// Low-cost metrics: mostly direct RSMI reads (~8–45 ms per hy-smi probe).
// Suitable when scrape interval is short or device count is large.
var metricsLevelLow = []string{
	"hcu_temp", "hcu_power_usage", "hcu_powercap", "hcu_sclk", "hcu_mclk", "hcu_utilizationrate",
	"hcu_usedmemory_bytes", "hcu_memorycap_bytes", "hcu_available_memory_bytes", "hcu_vram_percent",
	"hcu_compute_unit_count", "hcu_compute_unit_remaining_count", "hcu_memory_remaining",
	"hcu_ce_count", "hcu_ue_count", "hcu_ce_count_total", "hcu_ue_count_total",
	"hcu_se_usage", "hcu_temp_mem", "hcu_temp_board", "hcu_sensor_temp", "hcu_throttle",
	"hcu_sclk_max", "hcu_mclk_max",
	"hcu_xhcl_link_up", "hcu_xhcl_link_state", "hcu_xhcl_error_status",
	"hcu_link_accessible", "hcu_memory_overdrive", "hcu_overdrive", "hcu_membank_ecc",
	"hcu_fan_level", "hcu_fan_percent", "hcu_fan_rpm",
	"hcu_umc_bw_read", "hcu_umc_bw_write", "hcu_umc_bw_read_write",
	"hcu_umc_chan_bw_read", "hcu_umc_chan_bw_write", "hcu_umc_chan_bw_read_write",
	"hcu_pcie_replay_count", "hcu_pcie_width", "hcu_pcie_clock",
	"hcu_perf_level",
	"hcu_temp_edge_current", "hcu_temp_edge_critical", "hcu_temp_edge_emergency",
	"hcu_sensor_temp_max", "hcu_sensor_temp_critical",
	"hcu_powercap_range_max", "hcu_powercap_range_min",
	"hcu_meminfo_used_bytes", "hcu_meminfo_total_bytes", "hcu_bad_pages",
	"hcu_ecc_enabled", "hcu_ras_block_enabled",
	"hcu_voltage_mv", "hcu_encrypted_status",
	"hcu_node_id", "hcu_numa_node", "hcu_numa_affinity",
	"vhcu_count", "vhcu_temp", "vhcu_sclk", "vhcu_utilizationrate",
	"vhcu_usedmemory_bytes", "vhcu_usedmemory_percent",
}

// Medium-cost extras (~45–350 ms): DF/Hylink/XHCL bandwidth sampling, health check, process metrics.
var metricsLevelMediumExtra = []string{
	"hcu_df_bw_read", "hcu_df_bw_write", "hcu_df_bw_read_write",
	"hcu_hylink_send", "hcu_hylink_recv", "hcu_hylink_send_recv", "hcu_xhcl_bw",
	"hcu_health_status",
	"hcu_process_vram_used_bytes", "hcu_process_sdma_used", "hcu_process_cu_occupancy",
	"hcu_process_pasid", "hcu_process_hcu_percent", "hcu_process_vram_usage_rate",
}

// High-cost extras (~1 s+ per device): fixed 1s RSMI windows or --sample-duration-ms sampling.
var metricsLevelHighExtra = []string{
	"hcu_pciebw_mb", "hcu_pcie_sent_mb", "hcu_pcie_receive_mb",
	"hcu_util_percent", "hcu_cu_usage",
	"hcu_sampled_usage", "hcu_cu_sampled_usage", "hcu_wave_sampled_usage",
	"hcu_cu_util", "hcu_wave_util",
}

var allInternalMetricNames = func() []string {
	names := make([]string, 0, len(metricsLevelLow)+len(metricsLevelMediumExtra)+len(metricsLevelHighExtra))
	names = append(names, metricsLevelLow...)
	names = append(names, metricsLevelMediumExtra...)
	names = append(names, metricsLevelHighExtra...)
	return names
}()

var internalMetricNameSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(allInternalMetricNames))
	for _, name := range allInternalMetricNames {
		set[name] = struct{}{}
	}
	return set
}()

func AllInternalMetricNames() []string {
	names := make([]string, len(allInternalMetricNames))
	copy(names, allInternalMetricNames)
	return names
}

func isValidInternalMetricName(name string) bool {
	_, ok := internalMetricNameSet[name]
	return ok
}

func copyMetricNames(src []string) []string {
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// MetricsByLevel returns the preset metric list for low / medium / high.
// medium = low ∪ medium-extra; high = all metrics.
func MetricsByLevel(level string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", MetricsLevelHigh:
		return AllInternalMetricNames(), nil
	case MetricsLevelMedium:
		names := make([]string, 0, len(metricsLevelLow)+len(metricsLevelMediumExtra))
		names = append(names, metricsLevelLow...)
		names = append(names, metricsLevelMediumExtra...)
		return names, nil
	case MetricsLevelLow:
		return copyMetricNames(metricsLevelLow), nil
	default:
		return nil, fmt.Errorf("invalid metrics-level %q: want low, medium, or high", level)
	}
}

// ResolveEnableMetrics parses metric names to enable. Empty input enables all metrics.
func ResolveEnableMetrics(enableMetricsStr string) []string {
	names := ParseStringToArray(enableMetricsStr, ",")
	if len(names) == 0 {
		return AllInternalMetricNames()
	}

	resolved := make([]string, 0, len(names))
	for _, name := range names {
		if !isValidInternalMetricName(name) {
			glog.Warningf("Unknown metric name: %s, skipping", name)
			continue
		}
		resolved = append(resolved, name)
	}
	return resolved
}

// ResolveEnabledMetrics resolves the final metric list.
// If enableMetricsStr is non-empty, it takes precedence over metrics-level.
// Otherwise metrics-level preset is used (default high = all metrics).
func ResolveEnabledMetrics(metricsLevel, enableMetricsStr string) ([]string, error) {
	if strings.TrimSpace(enableMetricsStr) != "" {
		if strings.TrimSpace(metricsLevel) != "" &&
			!strings.EqualFold(strings.TrimSpace(metricsLevel), MetricsLevelHigh) {
			glog.Warningf("--enable-metrics is set; ignoring --metrics-level=%s", metricsLevel)
		}
		return ResolveEnableMetrics(enableMetricsStr), nil
	}
	return MetricsByLevel(metricsLevel)
}

func GetResourceNamePrefixList() []string {
	info, err := dcgm.GetDeviceInfo(0)
	if err != nil {
		glog.Errorf("Get device info err: %v", err)
	}
	cu_string := strconv.Itoa(info.ComputeUnitCount) + "CU"
	mem_string := strconv.Itoa(int(math.Round(float64(info.GlobalMemSize)/(1024*1024*1024)))) + "G"

	return []string{"hcu", info.Name, strings.ReplaceAll(info.Name, "_", "-") + "_" + mem_string + "_" + cu_string}
}
