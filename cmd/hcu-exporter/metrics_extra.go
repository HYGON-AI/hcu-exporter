// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/HYGON-AI/hcu-dcgm/v3/pkg/dcgm"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

// primaryTempSensors are the four sensors covered by GetDeviceTemperatureInfo
// (CURRENT) and used for MAX/CRITICAL via GetTempBySensor.
var primaryTempSensors = []struct {
	id   int
	name string
}{
	{0, "edge"},
	{1, "junction"},
	{2, "mem"},
	{11, "core"},
}

var memBankNumericFieldPattern = regexp.MustCompile(`(?i)^([a-zA-Z0-9_]+)[=:\s]+(-?[0-9]+(?:\.[0-9]+)?)`)

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func metricEnabled(name string) bool {
	for _, m := range metricsList {
		if strings.EqualFold(m, name) {
			return true
		}
	}
	return false
}

func parseMemBankNumericFields(properties string) map[string]float64 {
	result := make(map[string]float64)
	for _, line := range strings.Split(properties, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := memBankNumericFieldPattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		value, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			continue
		}
		result[strings.ToLower(matches[1])] = value
	}
	return result
}

// collectExtraDeviceMetrics fills metrics that need multi-value or multi-call collection.
func collectExtraDeviceMetrics(dvInd int, deviceInfos []dcgm.DeviceInfo, out map[string]float64) {
	collectSensorTempMetrics(dvInd, out)
	collectThrottleMetrics(dvInd, out)
	collectXhclLinkMetrics(dvInd, out)
	collectLinkAccessibleMetrics(dvInd, deviceInfos, out)
	collectMemBankEccMetrics(dvInd, out)
	collectUMCBandwidthMetrics(dvInd, out)
	collectPCIeLinkMetrics(dvInd, out)
	collectPerfLevelMetrics(dvInd, out)
	collectFanMetrics(dvInd, out)
	collectPowerCapRangeMetrics(dvInd, out)
	collectMemInfoMetrics(dvInd, out)
	collectBadPagesMetrics(dvInd, out)
	collectRasBlockEnabledMetrics(dvInd, out)
	collectXhclBandwidthMetrics(dvInd, out)
	collectNumaMetrics(dvInd, out)
}

func collectSensorTempMetrics(dvInd int, out map[string]float64) {
	needSensorCurrent := metricEnabled("hcu_sensor_temp")
	needSensorMax := metricEnabled("hcu_sensor_temp_max")
	needSensorCritical := metricEnabled("hcu_sensor_temp_critical")
	if !needSensorCurrent && !needSensorMax && !needSensorCritical {
		return
	}
	// CURRENT: one call covers EDGE/JUNCTION/MEMORY/CORE (partial failure still
	// returns successful fields; failed fields are zero and joined into err).
	if needSensorCurrent {
		temps, err := dcgm.GetDeviceTemperatureInfo(dvInd)
		if err != nil {
			glog.V(5).Infof("GetDeviceTemperatureInfo(%d) partial error: %v", dvInd, err)
		}
		out["hcu_sensor_temp-edge"] = temps.Edge
		out["hcu_sensor_temp-junction"] = temps.Junction
		out["hcu_sensor_temp-mem"] = temps.Memory
		out["hcu_sensor_temp-core"] = temps.Core
	}

	// MAX/CRITICAL are not provided by GetDeviceTemperatureInfo; keep
	// GetTempBySensor only for the same four primary sensors.
	if !needSensorMax && !needSensorCritical {
		return
	}
	for _, sensor := range primaryTempSensors {
		if needSensorMax {
			temp, err := dcgm.GetTempBySensor(dvInd, sensor.id, dcgm.RSMI_TEMP_MAX)
			if err != nil {
				glog.V(5).Infof("GetTempBySensor(%d,%s,max) error: %v", dvInd, sensor.name, err)
			} else {
				out["hcu_sensor_temp_max-"+sensor.name] = temp
			}
		}
		if needSensorCritical {
			temp, err := dcgm.GetTempBySensor(dvInd, sensor.id, dcgm.RSMI_TEMP_CRITICAL)
			if err != nil {
				glog.V(5).Infof("GetTempBySensor(%d,%s,critical) error: %v", dvInd, sensor.name, err)
			} else {
				out["hcu_sensor_temp_critical-"+sensor.name] = temp
			}
		}
	}
}

func collectThrottleMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_throttle") {
		return
	}
	flags, err := dcgm.DevThrottleFlags(dvInd)
	if err != nil {
		glog.Errorf("Get ThrottleFlags error: %v", err)
		return
	}
	out["hcu_throttle-thermal"] = boolToFloat(flags.Thermal)
	out["hcu_throttle-power"] = boolToFloat(flags.Power)
	out["hcu_throttle-slowdown"] = boolToFloat(flags.Slowdown)
	out["hcu_throttle-board_limit"] = boolToFloat(flags.BoardLimit)
}

func collectXhclLinkMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_xhcl_link_up") && !metricEnabled("hcu_xhcl_link_state") {
		return
	}
	states, err := dcgm.XhclLinkStates(dvInd)
	if err != nil {
		glog.Errorf("Get XhclLinkStates error: %v", err)
		return
	}
	for _, st := range states {
		linkKey := strconv.Itoa(st.LinkID)
		if metricEnabled("hcu_xhcl_link_up") {
			out["hcu_xhcl_link_up-"+linkKey] = boolToFloat(st.Up)
		}
		if metricEnabled("hcu_xhcl_link_state") {
			out["hcu_xhcl_link_state-"+linkKey] = float64(st.State)
		}
	}
}

func collectLinkAccessibleMetrics(dvInd int, deviceInfos []dcgm.DeviceInfo, out map[string]float64) {
	if !metricEnabled("hcu_link_accessible") {
		return
	}
	for _, dst := range deviceInfos {
		if dst.DvInd == dvInd {
			continue
		}
		ok, err := dcgm.IsP2PAccessible(dvInd, dst.DvInd)
		if err != nil {
			glog.V(5).Infof("IsP2PAccessible(%d,%d) error: %v", dvInd, dst.DvInd, err)
			continue
		}
		out["hcu_link_accessible-"+strconv.Itoa(dst.DvInd)] = boolToFloat(ok)
	}
}

func collectMemBankEccMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_membank_ecc") {
		return
	}
	banks, err := dcgm.MemBankEccInfo(dvInd)
	if err != nil {
		glog.V(5).Infof("MemBankEccInfo(%d) error: %v", dvInd, err)
		return
	}
	for bank, props := range banks {
		fields := parseMemBankNumericFields(props)
		if len(fields) == 0 {
			out["hcu_membank_ecc-"+bank+"|present"] = 1
			continue
		}
		for field, value := range fields {
			out["hcu_membank_ecc-"+bank+"|"+field] = value
		}
	}
}

func collectUMCBandwidthMetrics(dvInd int, out map[string]float64) {
	needUMC := metricEnabled("hcu_umc_bw_read") || metricEnabled("hcu_umc_bw_write") || metricEnabled("hcu_umc_bw_read_write") ||
		metricEnabled("hcu_umc_chan_bw_read") || metricEnabled("hcu_umc_chan_bw_write") || metricEnabled("hcu_umc_chan_bw_read_write")
	if !needUMC {
		return
	}
	info, err := dcgm.UMCBandwidth(dvInd, dcgm.MAX_UMC_CHAN_NUM, 10)
	if err != nil {
		glog.Errorf("Get UMCBandwidth error: %v", err)
		return
	}
	var readSum, writeSum, rwSum float64
	for i := 0; i < dcgm.MAX_UMC_CHAN_NUM; i++ {
		readSum += info.ReadBW[i]
		writeSum += info.WriteBW[i]
		rwSum += info.ReadWriteBW[i]
		chanKey := strconv.Itoa(i)
		if metricEnabled("hcu_umc_chan_bw_read") {
			out["hcu_umc_chan_bw_read-"+chanKey] = info.ReadBW[i]
		}
		if metricEnabled("hcu_umc_chan_bw_write") {
			out["hcu_umc_chan_bw_write-"+chanKey] = info.WriteBW[i]
		}
		if metricEnabled("hcu_umc_chan_bw_read_write") {
			out["hcu_umc_chan_bw_read_write-"+chanKey] = info.ReadWriteBW[i]
		}
	}
	if metricEnabled("hcu_umc_bw_read") {
		out["hcu_umc_bw_read"] = readSum
	}
	if metricEnabled("hcu_umc_bw_write") {
		out["hcu_umc_bw_write"] = writeSum
	}
	if metricEnabled("hcu_umc_bw_read_write") {
		out["hcu_umc_bw_read_write"] = rwSum
	}
}

func collectPCIeLinkMetrics(dvInd int, out map[string]float64) {
	if metricEnabled("hcu_pcie_width") || metricEnabled("hcu_pcie_clock") {
		bw, err := dcgm.DevPciBandwidth(dvInd)
		if err != nil {
			glog.Errorf("Get DevPciBandwidth error: %v", err)
		} else {
			idx := int(bw.TransferRate.Current)
			if idx >= 0 && idx < len(bw.TransferRate.Frequency) {
				if metricEnabled("hcu_pcie_clock") {
					out["hcu_pcie_clock"] = float64(bw.TransferRate.Frequency[idx])
				}
				if metricEnabled("hcu_pcie_width") {
					out["hcu_pcie_width"] = float64(bw.Lanes[idx])
				}
			}
		}
	}

	if !metricEnabled("hcu_pcie_replay_count") {
		return
	}
	infos, err := dcgm.ShowPcieReplayCount([]int{dvInd})
	if err != nil {
		glog.Errorf("Get PcieReplayCount error: %v", err)
	} else if len(infos) > 0 {
		out["hcu_pcie_replay_count"] = float64(infos[0].Count)
	}
}

func collectPerfLevelMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_perf_level") {
		return
	}
	level, err := dcgm.PerfLevel(dvInd)
	if err != nil {
		glog.Errorf("Get PerfLevel error: %v", err)
	} else if level != "" {
		out["hcu_perf_level-"+strings.ToLower(level)] = 1
	}
}

func collectFanMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_fan_level") && !metricEnabled("hcu_fan_percent") {
		return
	}
	level, percent, err := dcgm.FanSpeedInfo(dvInd)
	if err != nil {
		glog.V(5).Infof("FanSpeedInfo(%d) error: %v", dvInd, err)
		return
	}
	if metricEnabled("hcu_fan_level") {
		out["hcu_fan_level"] = float64(level)
	}
	if metricEnabled("hcu_fan_percent") {
		out["hcu_fan_percent"] = percent
	}
}

func collectPowerCapRangeMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_powercap_range_max") && !metricEnabled("hcu_powercap_range_min") {
		return
	}
	powerMax, powerMin, err := dcgm.DevPowerCapRange(dvInd)
	if err != nil {
		glog.Errorf("Get DevPowerCapRange error: %v", err)
		return
	}
	// DCGM returns microwatts; convert to Watts to align with hcu_powercap.
	if metricEnabled("hcu_powercap_range_max") {
		out["hcu_powercap_range_max"] = float64(powerMax) / 1e6
	}
	if metricEnabled("hcu_powercap_range_min") {
		out["hcu_powercap_range_min"] = float64(powerMin) / 1e6
	}
}

func collectMemInfoMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_meminfo_used_bytes") && !metricEnabled("hcu_meminfo_total_bytes") {
		return
	}
	for _, memType := range []string{"vram", "vis_vram", "gtt"} {
		used, total, err := dcgm.MemInfo(dvInd, memType)
		if err != nil {
			glog.V(5).Infof("MemInfo(%d,%s) error: %v", dvInd, memType, err)
			continue
		}
		if metricEnabled("hcu_meminfo_used_bytes") {
			out["hcu_meminfo_used_bytes-"+memType] = float64(used)
		}
		if metricEnabled("hcu_meminfo_total_bytes") {
			out["hcu_meminfo_total_bytes-"+memType] = float64(total)
		}
	}
}

func collectBadPagesMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_bad_pages") {
		return
	}
	records, err := dcgm.MemoryReservedPages(dvInd)
	if err != nil {
		glog.V(5).Infof("MemoryReservedPages(%d) error: %v", dvInd, err)
		return
	}
	counts := map[string]float64{
		"reserved":     0,
		"pending":      0,
		"unreservable": 0,
		"total":        float64(len(records)),
	}
	for _, rec := range records {
		status := dcgm.MemoryPageStatusStr[rec.Status]
		if status == "" {
			status = "unknown"
		}
		counts[status]++
	}
	for status, count := range counts {
		out["hcu_bad_pages-"+status] = count
	}
}

func collectRasBlockEnabledMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_ras_block_enabled") {
		return
	}
	blockInfos, err := dcgm.EccBlocksInfo(dvInd)
	if err != nil {
		glog.Errorf("Get EccBlocksInfo for enabled flag error: %v", err)
		return
	}
	for _, blockInfo := range blockInfos {
		out["hcu_ras_block_enabled-"+blockInfo.Block] = boolToFloat(strings.EqualFold(blockInfo.State, "ENABLED"))
	}
}

func collectXhclBandwidthMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_xhcl_bw") {
		return
	}
	recvInfo, recvErr := dcgm.XHCLBandwidth(dvInd, 0xff, 0, 10)
	sendInfo, sendErr := dcgm.XHCLBandwidth(dvInd, 0xff, 1, 10)
	if recvErr != nil {
		glog.V(5).Infof("XHCLBandwidth recv(%d) error: %v", dvInd, recvErr)
	}
	if sendErr != nil {
		glog.V(5).Infof("XHCLBandwidth send(%d) error: %v", dvInd, sendErr)
	}
	for linkID := 0; linkID < dcgm.MAX_XHCL_LINK_NUM; linkID++ {
		linkKey := strconv.Itoa(linkID)
		if recvErr == nil {
			out["hcu_xhcl_bw-"+linkKey+"|recv"] = recvInfo.Bw[linkID]
		}
		if sendErr == nil {
			out["hcu_xhcl_bw-"+linkKey+"|send"] = sendInfo.Bw[linkID]
		}
	}
}

func collectNumaMetrics(dvInd int, out map[string]float64) {
	if !metricEnabled("hcu_numa_node") && !metricEnabled("hcu_numa_affinity") {
		return
	}
	numaInfos, err := dcgm.ShowNumaTopology([]int{dvInd})
	if err != nil {
		glog.Errorf("ShowNumaTopology error: %v", err)
		return
	}
	if len(numaInfos) == 0 {
		return
	}
	if metricEnabled("hcu_numa_node") {
		out["hcu_numa_node"] = float64(numaInfos[0].NumaNode)
	}
	if metricEnabled("hcu_numa_affinity") {
		out["hcu_numa_affinity"] = float64(numaInfos[0].NumaAffinity)
	}
}

// collectHealthMetrics calls HCUHealthCheck once per scrape and maps results onto devices.
func collectHealthMetrics(deviceInfos []dcgm.DeviceInfo, metricsHCUCollectMap map[int]map[string]float64) {
	if !metricEnabled("hcu_health_status") {
		return
	}

	statuses, err := dcgm.HCUHealthCheck()
	if err != nil {
		glog.Errorf("HCUHealthCheck error: %v", err)
		return
	}

	byIndex := make(map[int]dcgm.HCUHealthStatus, len(statuses))
	byBusID := make(map[string]dcgm.HCUHealthStatus, len(statuses))
	for _, st := range statuses {
		byIndex[int(st.HCU)] = st
		if st.BusId != "" {
			byBusID[normalizeBusID(st.BusId)] = st
		}
	}

	for _, info := range deviceInfos {
		if _, exists := metricsHCUCollectMap[info.DvInd]; !exists {
			metricsHCUCollectMap[info.DvInd] = make(map[string]float64)
		}
		st, ok := byIndex[info.DvInd]
		if !ok {
			st, ok = byBusID[normalizeBusID(info.PciBusNumber)]
		}
		if !ok {
			glog.V(5).Infof("HCUHealthCheck: no result for device %d (%s)", info.DvInd, info.PciBusNumber)
			continue
		}
		status := st.Status
		if status == "" {
			status = "Unknown"
		}
		metricsHCUCollectMap[info.DvInd]["hcu_health_status-"+status] = healthStatusValue(status)
	}
}

func normalizeBusID(busID string) string {
	return strings.ToLower(strings.TrimSpace(busID))
}

func healthStatusValue(status string) float64 {
	if strings.EqualFold(status, "Healthy") {
		return 1
	}
	return 0
}

func collectProcessMetrics(deviceInfos []dcgm.DeviceInfo, metricsHCUCollectMap map[int]map[string]float64) {
	needProcessHCUInfo := metricEnabled("hcu_process_vram_used_bytes") ||
		metricEnabled("hcu_process_sdma_used") ||
		metricEnabled("hcu_process_cu_occupancy") ||
		metricEnabled("hcu_process_pasid") ||
		metricEnabled("hcu_process_hcu_percent") ||
		metricEnabled("hcu_process_vram_usage_rate")
	if !needProcessHCUInfo {
		return
	}

	deviceSet := make(map[int]struct{}, len(deviceInfos))
	deviceMemoryTotal := make(map[int]float64, len(deviceInfos))
	for _, info := range deviceInfos {
		deviceSet[info.DvInd] = struct{}{}
		deviceMemoryTotal[info.DvInd] = info.MemoryTotal
		if _, exists := metricsHCUCollectMap[info.DvInd]; !exists {
			metricsHCUCollectMap[info.DvInd] = make(map[string]float64)
		}
	}

	processes, err := dcgm.ProcessHCUInfo()
	if err != nil {
		if len(processes) == 0 {
			glog.Errorf("ProcessHCUInfo error: %v", err)
			return
		}
		// Partial results may still be returned; keep usable entries.
		glog.V(5).Infof("ProcessHCUInfo partial error: %v", err)
	}

	for _, proc := range processes {
		recordProcessMetrics(proc, deviceSet, deviceMemoryTotal, metricsHCUCollectMap)
	}
}

func processVramUsageRatePercent(vramUsed uint64, memoryTotal float64) float64 {
	if memoryTotal <= 0 {
		return 0
	}
	return float64(vramUsed) / memoryTotal * 100
}

func recordProcessMetrics(proc dcgm.Process, deviceSet map[int]struct{}, deviceMemoryTotal map[int]float64, metricsHCUCollectMap map[int]map[string]float64) {
	name := proc.ProcessName
	if name == "" {
		name = "unknown"
	}
	keySuffix := strconv.FormatUint(uint64(proc.ProcessID), 10) + "|" + name

	for _, minor := range proc.MinorNumbers {
		if _, ok := deviceSet[minor]; !ok {
			continue
		}
		if metricEnabled("hcu_process_vram_used_bytes") {
			metricsHCUCollectMap[minor]["hcu_process_vram_used_bytes-"+keySuffix] = float64(proc.VramUsage)
		}
		if metricEnabled("hcu_process_sdma_used") {
			metricsHCUCollectMap[minor]["hcu_process_sdma_used-"+keySuffix] = float64(proc.SdmaUsage)
		}
		if metricEnabled("hcu_process_cu_occupancy") {
			metricsHCUCollectMap[minor]["hcu_process_cu_occupancy-"+keySuffix] = float64(proc.CuOccupancy)
		}
		if metricEnabled("hcu_process_pasid") {
			metricsHCUCollectMap[minor]["hcu_process_pasid-"+keySuffix] = float64(proc.Pasid)
		}
		if metricEnabled("hcu_process_hcu_percent") {
			metricsHCUCollectMap[minor]["hcu_process_hcu_percent-"+keySuffix] = float64(proc.CuOccupancy)
		}
		if metricEnabled("hcu_process_vram_usage_rate") {
			metricsHCUCollectMap[minor]["hcu_process_vram_usage_rate-"+keySuffix] = processVramUsageRatePercent(proc.VramUsage, deviceMemoryTotal[minor])
		}
	}
}

func clearEphemeralLabels(labels prometheus.Labels) {
	for _, key := range []string{
		"link_id", "block_type", "se_id", "sensor_type", "throttle_type",
		"dst_minor_number", "bank", "field", "level", "pid", "process_name",
		"mem_type", "page_status", "chan_id", "direction", "health",
	} {
		delete(labels, key)
	}
}

// simpleExtraMetricPrefixes maps compound metric keys to (exported name, label key).
// Longer prefixes must appear before shorter ones that share a common stem.
var simpleExtraMetricPrefixes = []struct {
	prefix   string
	metric   string
	labelKey string
}{
	{"hcu_health_status-", "hcu_health_status", "health"},
	{"hcu_sensor_temp_max-", "hcu_sensor_temp_max", "sensor_type"},
	{"hcu_sensor_temp_critical-", "hcu_sensor_temp_critical", "sensor_type"},
	{"hcu_sensor_temp-", "hcu_sensor_temp", "sensor_type"},
	{"hcu_throttle-", "hcu_throttle", "throttle_type"},
	{"hcu_xhcl_link_up-", "hcu_xhcl_link_up", "link_id"},
	{"hcu_xhcl_link_state-", "hcu_xhcl_link_state", "link_id"},
	{"hcu_link_accessible-", "hcu_link_accessible", "dst_minor_number"},
	{"hcu_perf_level-", "hcu_perf_level", "level"},
	{"hcu_meminfo_used_bytes-", "hcu_meminfo_used_bytes", "mem_type"},
	{"hcu_meminfo_total_bytes-", "hcu_meminfo_total_bytes", "mem_type"},
	{"hcu_bad_pages-", "hcu_bad_pages", "page_status"},
	{"hcu_ras_block_enabled-", "hcu_ras_block_enabled", "block_type"},
	{"hcu_umc_chan_bw_read_write-", "hcu_umc_chan_bw_read_write", "chan_id"},
	{"hcu_umc_chan_bw_read-", "hcu_umc_chan_bw_read", "chan_id"},
	{"hcu_umc_chan_bw_write-", "hcu_umc_chan_bw_write", "chan_id"},
}

var processMetricPrefixes = []string{
	"hcu_process_vram_used_bytes-",
	"hcu_process_sdma_used-",
	"hcu_process_cu_occupancy-",
	"hcu_process_pasid-",
	"hcu_process_hcu_percent-",
	"hcu_process_vram_usage_rate-",
}

func setExtraMetricValue(metrics string, labels prometheus.Labels, value float64) bool {
	for _, h := range simpleExtraMetricPrefixes {
		if strings.HasPrefix(metrics, h.prefix) {
			labels[h.labelKey] = strings.TrimPrefix(metrics, h.prefix)
			setMetricValue(h.metric, labels, value)
			return true
		}
	}
	if setCompoundExtraMetric(metrics, labels, value) {
		return true
	}
	return setProcessExtraMetric(metrics, labels, value)
}

func setCompoundExtraMetric(metrics string, labels prometheus.Labels, value float64) bool {
	if strings.HasPrefix(metrics, "hcu_xhcl_bw-") {
		rest := strings.TrimPrefix(metrics, "hcu_xhcl_bw-")
		parts := strings.SplitN(rest, "|", 2)
		if len(parts) != 2 {
			return false
		}
		labels["link_id"] = parts[0]
		labels["direction"] = parts[1]
		setMetricValue("hcu_xhcl_bw", labels, value)
		return true
	}
	if strings.HasPrefix(metrics, "hcu_membank_ecc-") {
		rest := strings.TrimPrefix(metrics, "hcu_membank_ecc-")
		parts := strings.SplitN(rest, "|", 2)
		if len(parts) != 2 {
			return false
		}
		labels["bank"] = parts[0]
		labels["field"] = parts[1]
		setMetricValue("hcu_membank_ecc", labels, value)
		return true
	}
	return false
}

func setProcessExtraMetric(metrics string, labels prometheus.Labels, value float64) bool {
	for _, prefix := range processMetricPrefixes {
		if !strings.HasPrefix(metrics, prefix) {
			continue
		}
		metricName := strings.TrimSuffix(prefix, "-")
		rest := strings.TrimPrefix(metrics, prefix)
		parts := strings.SplitN(rest, "|", 2)
		if len(parts) != 2 {
			return false
		}
		labels["pid"] = parts[0]
		labels["process_name"] = parts[1]
		setMetricValue(metricName, labels, value)
		return true
	}
	return false
}
