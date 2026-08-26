// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import "C"
import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	_ "strconv"
	"strings"
	"time"

	"github.com/HYGON-AI/hcu-exporter/v3/pkg/podresources"
	"github.com/HYGON-AI/hcu-exporter/v3/pkg/util"

	"github.com/HYGON-AI/hcu-dcgm/v3/pkg/dcgm"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	pulse, portFlag      int
	hylinkDetailFlag     bool
	connectK8sFlag       bool
	sampleDurationMsFlag int
	allowedIPsFlag       string
	allowedIPs           []string
	allowedCIDRs         []*net.IPNet
	socket               = "/var/lib/kubelet/pod-resources/kubelet.sock"

	timeout = 10 * time.Second

	resources           = util.GetResourceNameList(false)
	vhcuDynamicResource = util.GetResourceNameList(true)

	vhcuShareResource = []string{
		"hygon.com/hcu-share",
	}

	maxSize = 1024 * 1024 * 16 // 16 Mb

	hcuLabels        = util.HCULabels
	vHcuLabels       = util.VHcuLabels
	hcuErrLabels     = util.HCUErrLabels
	hylinkLabels     = util.HylinkLabels
	seLabels         = util.SELabels
	sensorLabels     = util.SensorLabels
	throttleLabels   = util.ThrottleLabels
	p2pLabels        = util.P2PLabels
	memBankLabels    = util.MemBankLabels
	perfLabels       = util.PerfLabels
	processLabels    = util.ProcessLabels
	memTypeLabels    = util.MemTypeLabels
	pageStatusLabels = util.PageStatusLabels
	chanLabels       = util.ChanLabels
	xhclBwLabels     = util.XhclBwLabels
	healthLabels     = util.HealthLabels
	metricsList      []string
	metricConfig     *util.MetricConfig
)

// 定义collector
var (
	hcuTemp                      *prometheus.GaugeVec
	hcuPowerUsage                *prometheus.GaugeVec
	hcuPowerCap                  *prometheus.GaugeVec
	hcuSclk                      *prometheus.GaugeVec
	hcuMclk                      *prometheus.GaugeVec
	hcuUtilizationRate           *prometheus.GaugeVec
	hcuUsedMemoryBytes           *prometheus.GaugeVec
	hcuMemoryCapBytes            *prometheus.GaugeVec
	hcuPcieBwMb                  *prometheus.GaugeVec
	hcuPcieSentMb                *prometheus.GaugeVec
	hcuPcieReceiveMb             *prometheus.GaugeVec
	hcuComputeUnitCount          *prometheus.GaugeVec
	hcuComputeUnitRemainingCount *prometheus.GaugeVec
	hcuMemoryRemaining           *prometheus.GaugeVec
	hcuCE                        *prometheus.GaugeVec
	hcuUE                        *prometheus.GaugeVec
	hcu_df_bw_read               *prometheus.GaugeVec
	hcu_df_bw_write              *prometheus.GaugeVec
	hcu_df_bw_read_write         *prometheus.GaugeVec
	hcu_hylink_send              *prometheus.GaugeVec
	hcu_hylink_recv              *prometheus.GaugeVec
	hcu_hylink_send_recv         *prometheus.GaugeVec
	vhcuCount                    *prometheus.GaugeVec
	hcuCuUsage                   *prometheus.GaugeVec
	hcuSampledUsage              *prometheus.GaugeVec
	hcuCuSampledUsage            *prometheus.GaugeVec
	hcuWaveSampledUsage          *prometheus.GaugeVec
	hcuSeUsage                   *prometheus.GaugeVec
	vhcuTemp                     *prometheus.GaugeVec
	vhcuSclk                     *prometheus.GaugeVec
	vhcuUtilizationRate          *prometheus.GaugeVec
	vhcuUsedMemoryBytes          *prometheus.GaugeVec
	vhcuUsedMemoryPercent        *prometheus.GaugeVec
	metricsMap                   map[string]*prometheus.GaugeVec
)

func newGaugeVec(name, help string, labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
}

func initMetrics(cfg *util.MetricConfig) {
	registerMetric := func(internalName, help string, baseLabels []string, dest **prometheus.GaugeVec) {
		displayName := cfg.DisplayName(internalName)
		displayLabels := cfg.LabelsForMetric(internalName, baseLabels)
		gv := newGaugeVec(displayName, help, displayLabels)
		if dest != nil {
			*dest = gv
		}
		metricsMap[internalName] = gv
	}

	metricsMap = make(map[string]*prometheus.GaugeVec)
	registerMetric("hcu_temp", "hcu metrics of gauge", hcuLabels, &hcuTemp)
	registerMetric("hcu_power_usage", "hcu metrics of gauge", hcuLabels, &hcuPowerUsage)
	registerMetric("hcu_powercap", "hcu metrics of gauge", hcuLabels, &hcuPowerCap)
	registerMetric("hcu_sclk", "hcu metrics of gauge", hcuLabels, &hcuSclk)
	registerMetric("hcu_mclk", "hcu metrics of gauge", hcuLabels, &hcuMclk)
	registerMetric("hcu_utilizationrate", "hcu metrics of gauge", hcuLabels, &hcuUtilizationRate)
	registerMetric("hcu_usedmemory_bytes", "hcu metrics of gauge", hcuLabels, &hcuUsedMemoryBytes)
	registerMetric("hcu_memorycap_bytes", "hcu metrics of gauge", hcuLabels, &hcuMemoryCapBytes)
	registerMetric("hcu_pciebw_mb", "hcu metrics of gauge", hcuLabels, &hcuPcieBwMb)
	registerMetric("hcu_pcie_sent_mb", "hcu metrics of gauge", hcuLabels, &hcuPcieSentMb)
	registerMetric("hcu_pcie_receive_mb", "hcu metrics of gauge", hcuLabels, &hcuPcieReceiveMb)
	registerMetric("hcu_compute_unit_count", "hcu metrics of gauge", hcuLabels, &hcuComputeUnitCount)
	registerMetric("hcu_compute_unit_remaining_count", "hcu metrics of gauge", hcuLabels, &hcuComputeUnitRemainingCount)
	registerMetric("hcu_memory_remaining", "hcu metrics of gauge", hcuLabels, &hcuMemoryRemaining)
	registerMetric("hcu_ce_count", "hcu metrics of gauge", hcuErrLabels, &hcuCE)
	registerMetric("hcu_ue_count", "hcu metrics of gauge", hcuErrLabels, &hcuUE)
	registerMetric("hcu_ce_count_total", "Total correctable ECC errors across all blocks", hcuLabels, nil)
	registerMetric("hcu_ue_count_total", "Total uncorrectable ECC errors across all blocks", hcuLabels, nil)
	registerMetric("hcu_df_bw_read", "hcu metrics of gauge", hcuLabels, &hcu_df_bw_read)
	registerMetric("hcu_df_bw_write", "hcu metrics of gauge", hcuLabels, &hcu_df_bw_write)
	registerMetric("hcu_df_bw_read_write", "hcu metrics of gauge", hcuLabels, &hcu_df_bw_read_write)
	registerMetric("hcu_hylink_send", "hcu metrics of gauge", hylinkLabels, &hcu_hylink_send)
	registerMetric("hcu_hylink_recv", "hcu metrics of gauge", hylinkLabels, &hcu_hylink_recv)
	registerMetric("hcu_hylink_send_recv", "hcu metrics of gauge", hylinkLabels, &hcu_hylink_send_recv)
	registerMetric("vhcu_count", "hcu metrics of gauge", hcuLabels, &vhcuCount)
	registerMetric("hcu_cu_usage", "HCU instantaneous CU usage rate (percent)", hcuLabels, &hcuCuUsage)
	registerMetric("hcu_sampled_usage", "HCU sampled usage rate within sample window (percent)", hcuLabels, &hcuSampledUsage)
	registerMetric("hcu_cu_sampled_usage", "Average CU sampled usage rate within sample window (percent)", hcuLabels, &hcuCuSampledUsage)
	registerMetric("hcu_wave_sampled_usage", "Average wave sampled usage rate within sample window (percent)", hcuLabels, &hcuWaveSampledUsage)
	registerMetric("hcu_se_usage", "Shader Engine instantaneous usage rate (percent)", seLabels, &hcuSeUsage)
	registerMetric("hcu_temp_mem", "HCU memory temperature (Celsius)", hcuLabels, nil)
	registerMetric("hcu_temp_board", "HCU board edge temperature (Celsius)", hcuLabels, nil)
	registerMetric("hcu_sensor_temp", "HCU temperature by sensor type from GetDeviceTemperatureInfo (Celsius)", sensorLabels, nil)
	registerMetric("hcu_throttle", "HCU throttle flag (1=active)", throttleLabels, nil)
	registerMetric("hcu_cu_util", "CU wave occupancy ratio within sample window (0-1)", hcuLabels, nil)
	registerMetric("hcu_wave_util", "Wave residency ratio within sample window (0-1)", hcuLabels, nil)
	registerMetric("hcu_sclk_max", "HCU GFX max clock (MHz)", hcuLabels, nil)
	registerMetric("hcu_mclk_max", "HCU memory max clock (MHz)", hcuLabels, nil)
	registerMetric("hcu_xhcl_link_up", "XHCL/HSL link up status (1=up)", hylinkLabels, nil)
	registerMetric("hcu_xhcl_link_state", "XHCL/HSL raw link state value", hylinkLabels, nil)
	registerMetric("hcu_available_memory_bytes", "Approximate available VRAM bytes", hcuLabels, nil)
	registerMetric("hcu_link_accessible", "P2P accessibility to destination device (1=accessible)", p2pLabels, nil)
	registerMetric("hcu_memory_overdrive", "Memory overdrive level percent", hcuLabels, nil)
	registerMetric("hcu_membank_ecc", "Memory bank ECC numeric fields from sysfs properties", memBankLabels, nil)
	registerMetric("hcu_fan_level", "Fan speed level", hcuLabels, nil)
	registerMetric("hcu_fan_percent", "Fan speed percent of max", hcuLabels, nil)
	registerMetric("hcu_fan_rpm", "Fan speed RPM", hcuLabels, nil)
	registerMetric("hcu_vram_percent", "Memory busy percent", hcuLabels, nil)
	registerMetric("hcu_util_percent", "Device busy percent", hcuLabels, nil)
	registerMetric("hcu_umc_bw_read", "UMC read bandwidth sum", hcuLabels, nil)
	registerMetric("hcu_umc_bw_write", "UMC write bandwidth sum", hcuLabels, nil)
	registerMetric("hcu_umc_bw_read_write", "UMC read-write bandwidth sum", hcuLabels, nil)
	registerMetric("hcu_pcie_replay_count", "PCIe replay count", hcuLabels, nil)
	registerMetric("hcu_pcie_width", "Negotiated PCIe link width (lanes)", hcuLabels, nil)
	registerMetric("hcu_pcie_clock", "Negotiated PCIe link speed", hcuLabels, nil)
	registerMetric("hcu_xhcl_error_status", "HSL error status code", hcuLabels, nil)
	registerMetric("hcu_perf_level", "Current performance level (info gauge, value=1)", perfLabels, nil)
	registerMetric("hcu_process_vram_used_bytes", "Process VRAM usage bytes from ProcessHCUInfo", processLabels, nil)
	registerMetric("hcu_process_sdma_used", "Process SDMA usage from ProcessHCUInfo", processLabels, nil)
	registerMetric("hcu_process_cu_occupancy", "Process CU occupancy from ProcessHCUInfo", processLabels, nil)
	registerMetric("hcu_process_pasid", "Process address space ID (PASID) from ProcessHCUInfo", processLabels, nil)
	registerMetric("hcu_process_hcu_percent", "Process GPU compute usage percent from ProcessInfoByPid", processLabels, nil)
	registerMetric("hcu_process_vram_usage_rate", "Process VRAM usage rate percent from ProcessInfoByPid", processLabels, nil)
	registerMetric("hcu_temp_edge_max", "HCU edge temperature max limit (Celsius)", hcuLabels, nil)
	registerMetric("hcu_temp_edge_critical", "HCU edge temperature critical limit (Celsius)", hcuLabels, nil)
	registerMetric("hcu_sensor_temp_max", "HCU sensor temperature max by sensor type (Celsius)", sensorLabels, nil)
	registerMetric("hcu_sensor_temp_critical", "HCU sensor temperature critical by sensor type (Celsius)", sensorLabels, nil)
	registerMetric("hcu_overdrive", "GFX overdrive level percent", hcuLabels, nil)
	registerMetric("hcu_powercap_range_max", "Configurable power cap maximum (Watts)", hcuLabels, nil)
	registerMetric("hcu_powercap_range_min", "Configurable power cap minimum (Watts)", hcuLabels, nil)
	registerMetric("hcu_meminfo_used_bytes", "Memory used bytes by memory type", memTypeLabels, nil)
	registerMetric("hcu_meminfo_total_bytes", "Memory total bytes by memory type", memTypeLabels, nil)
	registerMetric("hcu_bad_pages", "Retired/reserved memory page count by status", pageStatusLabels, nil)
	registerMetric("hcu_ecc_enabled", "ECC enabled GPU block bitmask", hcuLabels, nil)
	registerMetric("hcu_ras_block_enabled", "ECC block enabled flag (1=enabled)", hcuErrLabels, nil)
	registerMetric("hcu_xhcl_bw", "XHCL bandwidth by link and direction", xhclBwLabels, nil)
	registerMetric("hcu_umc_chan_bw_read", "UMC read bandwidth by channel", chanLabels, nil)
	registerMetric("hcu_umc_chan_bw_write", "UMC write bandwidth by channel", chanLabels, nil)
	registerMetric("hcu_umc_chan_bw_read_write", "UMC read-write bandwidth by channel", chanLabels, nil)
	registerMetric("hcu_voltage_mv", "Device voltage in millivolts", hcuLabels, nil)
	registerMetric("hcu_encrypted_status", "Encryption VM status (1=enabled)", hcuLabels, nil)
	registerMetric("hcu_node_id", "KFD node ID", hcuLabels, nil)
	registerMetric("hcu_numa_node", "NUMA node of the device", hcuLabels, nil)
	registerMetric("hcu_numa_affinity", "NUMA affinity of the device", hcuLabels, nil)
	registerMetric("hcu_health_status", "HCU overall health status from HCUHealthCheck (1=Healthy, 0=otherwise)", healthLabels, nil)
	registerMetric("vhcu_temp", "vhcu metrics of gauge", vHcuLabels, &vhcuTemp)
	registerMetric("vhcu_sclk", "vhcu metrics of gauge", vHcuLabels, &vhcuSclk)
	registerMetric("vhcu_utilizationrate", "vhcu metrics of gauge", vHcuLabels, &vhcuUtilizationRate)
	registerMetric("vhcu_usedmemory_bytes", "vhcu metrics of gauge", vHcuLabels, &vhcuUsedMemoryBytes)
	registerMetric("vhcu_usedmemory_percent", "vhcu metrics of gauge", vHcuLabels, &vhcuUsedMemoryPercent)
}

func setMetricValue(internalMetric string, labels prometheus.Labels, value float64) {
	if collector, ok := metricsMap[internalMetric]; ok {
		collector.With(metricConfig.RemapLabels(labels)).Set(value)
	}
}

var (

	//HCU metrics collect function map
	hcuFunctionMap = map[string]func(idx int) float64{
		"hcu_temp": func(idx int) float64 {
			temperature, err := dcgm.Temperature(idx)
			if err != nil {
				glog.Errorf("Get Temperature error: %v", err)
			}
			return temperature
		},
		"hcu_power_usage": func(idx int) float64 {
			power, err := dcgm.Power(idx)
			if err != nil {
				glog.Errorf("Get Power error: %v", err)
			}
			return float64(power)
		},
		"hcu_powercap": func(idx int) float64 {
			powercap, err := dcgm.MaxPower(idx)
			if err != nil {
				glog.Errorf("Get Power Capacity error: %v", err)
			}
			return float64(powercap)
		},
		"hcu_utilizationrate": func(idx int) float64 {
			//CU Utilizationrate
			utilizationrate, err := dcgm.HCUUse(idx)
			if err != nil {
				glog.Errorf("Get Utilizationrate error: %v", err)
			}
			return float64(utilizationrate)
		},
		"hcu_usedmemory_bytes": func(idx int) float64 {
			memUsed, err := dcgm.MemoryUsed(idx)
			if err != nil {
				glog.Errorf("Get MemInfo error: %v", err)
			}
			return memUsed
		},
		"hcu_memorycap_bytes": func(idx int) float64 {
			memTotal, err := dcgm.MemoryTotal(idx)
			if err != nil {
				glog.Errorf("Get MemInfo error: %v", err)
			}
			return memTotal
		},
		"hcu_sclk": func(idx int) float64 {
			clk, err := dcgm.HCUClk(idx)
			if err != nil {
				glog.Errorf("Get Sclk error: %v", err)
			}
			return clk
		},
		"hcu_mclk": func(idx int) float64 {
			clk, err := dcgm.HCUMclk(idx)
			if err != nil {
				glog.Errorf("Get Mclk error: %v", err)
			}
			return clk
		},
		"hcu_compute_unit_count": func(idx int) float64 {
			deviceInfo, err := dcgm.GetDeviceInfo(idx)
			if err != nil {
				glog.Errorf("Get CU error: %v", err)
			}
			return float64(deviceInfo.ComputeUnitCount)
		},
		"hcu_compute_unit_remaining_count": func(idx int) float64 {
			cus, _, err := dcgm.DeviceRemainingInfo(idx)
			if err != nil {
				glog.Errorf("Get CU Remaining error: %v", err)
			}
			return float64(cus)
		},
		"hcu_memory_remaining": func(idx int) float64 {
			_, memories, err := dcgm.DeviceRemainingInfo(idx)
			if err != nil {
				glog.Errorf("Get Memory Remaining error: %v", err)
			}
			return float64(memories)
		},
		"hcu_df_bw_read": func(idx int) float64 {
			bandwidth, err := dcgm.DFBandwidth(idx, dcgm.RSMI_DF_BW_TYPE_ALL)
			if err != nil {
				glog.Errorf("Get DF BW error: %v", err)
			}
			return bandwidth.ReadBW
		},
		"hcu_df_bw_write": func(idx int) float64 {
			bandwidth, err := dcgm.DFBandwidth(idx, dcgm.RSMI_DF_BW_TYPE_ALL)
			if err != nil {
				glog.Errorf("Get DF BW error: %v", err)
			}
			return bandwidth.WriteBW
		},
		"hcu_df_bw_read_write": func(idx int) float64 {
			bandwidth, err := dcgm.DFBandwidth(idx, dcgm.RSMI_DF_BW_TYPE_ALL)
			if err != nil {
				glog.Errorf("Get DF BW error: %v", err)
			}
			return bandwidth.ReadWriteBW
		},
		"vhcu_count": func(idx int) float64 {
			vdeviceCount, _, err := dcgm.VDeviceByDvInd(idx)
			if err != nil {
				glog.Errorf("Get vHCU Count error: %v", err)
			}
			return float64(vdeviceCount)
		},
		"hcu_cu_usage": func(idx int) float64 {
			rate, err := dcgm.HCUCuUsage(idx)
			if err != nil {
				glog.Errorf("Get HCU CU Usage error: %v", err)
			}
			return rate
		},
		"hcu_sampled_usage": func(idx int) float64 {
			rate, err := dcgm.HCUSampledUsage(idx, sampleDurationMsFlag)
			if err != nil {
				glog.Errorf("Get HCU Sampled Usage error: %v", err)
			}
			return rate
		},
		"hcu_cu_sampled_usage": func(idx int) float64 {
			rate, err := dcgm.HCUCUSampledUsage(idx, sampleDurationMsFlag)
			if err != nil {
				glog.Errorf("Get HCU CU Sampled Usage error: %v", err)
			}
			return rate
		},
		"hcu_wave_sampled_usage": func(idx int) float64 {
			rate, err := dcgm.HCUWaveSampledUsage(idx, sampleDurationMsFlag)
			if err != nil {
				glog.Errorf("Get HCU Wave Sampled Usage error: %v", err)
			}
			return rate
		},
		"hcu_temp_mem": func(idx int) float64 {
			temp, err := dcgm.MemoryTemperature(idx)
			if err != nil {
				glog.Errorf("Get Memory Temperature error: %v", err)
			}
			return temp
		},
		"hcu_temp_board": func(idx int) float64 {
			temp, err := dcgm.BoardTemperature(idx)
			if err != nil {
				glog.Errorf("Get Board Temperature error: %v", err)
			}
			return temp
		},
		"hcu_cu_util": func(idx int) float64 {
			rate, err := dcgm.DevCuUtil(idx, sampleDurationMsFlag)
			if err != nil {
				glog.Errorf("Get DevCuUtil error: %v", err)
			}
			return float64(rate)
		},
		"hcu_wave_util": func(idx int) float64 {
			rate, err := dcgm.DevWaveUtil(idx, sampleDurationMsFlag)
			if err != nil {
				glog.Errorf("Get DevWaveUtil error: %v", err)
			}
			return float64(rate)
		},
		"hcu_sclk_max": func(idx int) float64 {
			clk, err := dcgm.DevGfxClockMax(idx)
			if err != nil {
				glog.Errorf("Get DevGfxClockMax error: %v", err)
			}
			return float64(clk)
		},
		"hcu_mclk_max": func(idx int) float64 {
			clk, err := dcgm.DevMemClockMax(idx)
			if err != nil {
				glog.Errorf("Get DevMemClockMax error: %v", err)
			}
			return float64(clk)
		},
		"hcu_available_memory_bytes": func(idx int) float64 {
			available, err := dcgm.MemoryAvailable(idx)
			if err != nil {
				glog.Errorf("Get MemoryAvailable error: %v", err)
			}
			return float64(available)
		},
		"hcu_memory_overdrive": func(idx int) float64 {
			level, err := dcgm.DevMemOverdriveLevelGet(idx)
			if err != nil {
				glog.Errorf("Get MemOverdrive error: %v", err)
			}
			return float64(level)
		},
		"hcu_fan_rpm": func(idx int) float64 {
			rpm, err := dcgm.DevFanRpms(idx)
			if err != nil {
				glog.Errorf("Get Fan RPM error: %v", err)
			}
			return float64(rpm)
		},
		"hcu_vram_percent": func(idx int) float64 {
			percent, err := dcgm.MemoryPercent(idx)
			if err != nil {
				glog.Errorf("Get MemoryPercent error: %v", err)
			}
			return float64(percent)
		},
		"hcu_util_percent": func(idx int) float64 {
			percent, err := dcgm.DevBusyPercent(idx)
			if err != nil {
				glog.Errorf("Get DevBusyPercent error: %v", err)
			}
			return percent
		},
		"hcu_xhcl_error_status": func(idx int) float64 {
			status, err := dcgm.HSLErrorStatus(idx)
			if err != nil {
				glog.Errorf("Get HSLErrorStatus error: %v", err)
			}
			return float64(status)
		},
		"hcu_temp_edge_max": func(idx int) float64 {
			temp, err := dcgm.GetTempByMetric(idx, dcgm.RSMI_TEMP_MAX)
			if err != nil {
				glog.Errorf("Get TempMax error: %v", err)
			}
			return temp
		},
		"hcu_temp_edge_critical": func(idx int) float64 {
			temp, err := dcgm.GetTempByMetric(idx, dcgm.RSMI_TEMP_CRITICAL)
			if err != nil {
				glog.Errorf("Get TempCritical error: %v", err)
			}
			return temp
		},
		"hcu_overdrive": func(idx int) float64 {
			od, err := dcgm.DevOverdriveLevelGet(idx)
			if err != nil {
				glog.Errorf("Get GFX Overdrive error: %v", err)
			}
			return float64(od)
		},
		"hcu_ecc_enabled": func(idx int) float64 {
			enabled, err := dcgm.EccEnabled(idx)
			if err != nil {
				glog.Errorf("Get EccEnabled error: %v", err)
			}
			return float64(enabled)
		},
		"hcu_voltage_mv": func(idx int) float64 {
			infos, err := dcgm.ShowVoltage([]int{idx})
			if err != nil {
				glog.Errorf("Get Voltage error: %v", err)
				return 0
			}
			if len(infos) == 0 {
				return 0
			}
			return float64(infos[0].Voltage)
		},
		"hcu_encrypted_status": func(idx int) float64 {
			status, err := dcgm.EncryptionVMStatus()
			if err != nil {
				glog.Errorf("Get EncryptionVMStatus error: %v", err)
			}
			return boolToFloat(status)
		},
		"hcu_node_id": func(idx int) float64 {
			nodeID, err := dcgm.DevNodeId(idx)
			if err != nil {
				glog.Errorf("Get DevNodeId error: %v", err)
			}
			return float64(nodeID)
		},
	}

	//vHCU metrics collect function map
	vhcuFunctionMap = map[string]func(vDevice dcgm.VDeviceInfo) float64{
		"vhcu_sclk": func(vDevice dcgm.VDeviceInfo) float64 {
			temperature, err := dcgm.HCUClk(vDevice.DvInd)
			if err != nil {
				glog.Errorf("Get vHCU Clock error: %v", err)
			}
			return temperature
		},
		"vhcu_temp": func(vDevice dcgm.VDeviceInfo) float64 {
			temperature, err := dcgm.Temperature(vDevice.DvInd)
			if err != nil {
				glog.Errorf("Get vHCU Temperature error: %v", err)
			}
			return temperature
		},
		"vhcu_utilizationrate": func(vDevice dcgm.VDeviceInfo) float64 {
			utilizationrate, err := dcgm.VDevBusyPercent(vDevice.VdvInd)
			if err != nil {
				glog.V(3).Infof("Get vHCU Utilizationrate error: %v", err)
			}
			return float64(utilizationrate)
		},
		"vhcu_usedmemory_bytes": func(vDevice dcgm.VDeviceInfo) float64 {
			vDeviceInfo, err := dcgm.VDeviceSingleInfo(vDevice.VdvInd)
			if err != nil {
				glog.V(3).Infof("Get vHCU Usedmemory error: %v", err)
			}
			return float64(vDeviceInfo.VMemoryUsed)
		},
		"vhcu_usedmemory_percent": func(vDevice dcgm.VDeviceInfo) float64 {
			vDeviceInfo, err := dcgm.VDeviceSingleInfo(vDevice.VdvInd)
			if err != nil {
				glog.V(3).Infof("Get vHCU Usedmemory error: %v", err)
			}
			return float64(vDeviceInfo.VMemoryUsed) / float64(vDeviceInfo.VMemoryTotal)
		},
	}
)

var podInfoMap = make(map[string]podresources.PodInfo)
var podHCUShareInfoMap = make(map[string]podresources.PodInfo)
var podHCUDynamicInfoMap = make(map[string]podresources.PodInfo)

var version = ""

func main() {
	c := cli.NewApp()
	c.Name = "Hygon HCU Exporter"
	c.Usage = "Hygon HCU Exporter"
	c.Version = version
	c.Before = beforeCLI
	c.Action = func(ctx *cli.Context) error {
		start()
		return nil
	}
	c.Flags = exporterFlags()

	err := c.Run(os.Args)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
}

// 指标采集启动函数
func start() {
	glog.V(2).Infof("🚀 🚀 🚀  HCU exporter start ...")

	_ = util.EnsureDirExists(podresources.VIRTUAL_HCU_CONF_DIR)

	err := dcgm.Init()
	if err != nil {
		glog.Errorf("Init HCU DCGM Error %v", err)
		os.Exit(1)
	}
	glog.V(2).Infoln("Init HCU DCGM successful")
	defer func() {
		err := dcgm.ShutDown()
		if err != nil {
			glog.Errorf("HCU exporter shutdown Error: %v ", err)
			return
		}
	}()

	initMetrics(metricConfig)

	// 这里用自定义注册表，可以使返回的数据比较简洁
	registry := prometheus.NewRegistry()

	// 根据参数注册指标
	for _, metricName := range metricsList {
		metricName = strings.TrimSpace(metricName)
		if collector, ok := metricsMap[metricName]; ok {
			registry.MustRegister(collector)
		} else {
			glog.Warningf("Unknown metric name: %s, skipping registration", metricName)
		}
	}

	recordMetrics()

	// 解析IP白名单
	parseAllowedIPs()

	port := fmt.Sprintf("%d", portFlag)
	glog.V(2).Infof("🚀 🚀 🚀  HCU exporter start on port %d ...", portFlag)

	metricsHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry})
	http.Handle("/metrics", ipWhitelistMiddleware(metricsHandler))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// 解析IP白名单参数
func parseAllowedIPs() {
	if allowedIPsFlag == "" {
		glog.V(2).Infof("No IP whitelist configured, all IPs are allowed")
		return
	}

	items := strings.Split(allowedIPsFlag, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			// CIDR格式
			_, cidr, err := net.ParseCIDR(item)
			if err != nil {
				glog.Errorf("Invalid CIDR format: %s, error: %v", item, err)
				continue
			}
			allowedCIDRs = append(allowedCIDRs, cidr)
			glog.V(2).Infof("Added allowed CIDR: %s", item)
		} else {
			// 单个IP
			ip := net.ParseIP(item)
			if ip == nil {
				glog.Errorf("Invalid IP format: %s", item)
				continue
			}
			allowedIPs = append(allowedIPs, item)
			glog.V(2).Infof("Added allowed IP: %s", item)
		}
	}
	glog.V(2).Infof("IP whitelist configured: %d IPs, %d CIDRs", len(allowedIPs), len(allowedCIDRs))
}

// IP白名单中间件
func ipWhitelistMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 如果没有配置白名单，允许所有IP访问
		if len(allowedIPs) == 0 && len(allowedCIDRs) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// 获取客户端IP
		clientIP := getClientIP(r)
		if clientIP == "" {
			glog.Warningf("Cannot determine client IP, denying access")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// 检查IP是否在白名单中
		if isIPAllowed(clientIP) {
			next.ServeHTTP(w, r)
			return
		}

		glog.Warningf("Access denied for IP: %s", clientIP)
		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}

// 获取客户端真实IP
func getClientIP(r *http.Request) string {
	// 优先从X-Forwarded-For获取
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// 从X-Real-IP获取
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// 从RemoteAddr获取
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// 判断IP是否在白名单中
func isIPAllowed(clientIP string) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	// 检查单个IP
	for _, allowedIP := range allowedIPs {
		if clientIP == allowedIP {
			return true
		}
	}

	// 检查CIDR
	for _, cidr := range allowedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// HCU物理卡指标重置
func collectorReset() {
	for name, gv := range metricsMap {
		if strings.HasPrefix(name, "vhcu_") && name != "vhcu_count" {
			continue
		}
		gv.Reset()
	}
}

// HCU虚拟卡指标重置
func vhcuCollectorReset() {
	for name, gv := range metricsMap {
		if strings.HasPrefix(name, "vhcu_") && name != "vhcu_count" {
			gv.Reset()
		}
	}
}

// k8s指标信息更新机制，只有所在节点pod有变换时才执行
func informerPodHandler(nodeName string, clientset *kubernetes.Clientset) {
	stopCh := make(chan struct{})
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()

	podInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = fieldSelector
				return clientset.CoreV1().Pods("").List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = fieldSelector
				return clientset.CoreV1().Pods("").Watch(context.TODO(), options)
			},
		},
		&v1.Pod{},
		10*time.Minute,
		cache.Indexers{},
	)

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if util.RequestsHCU(pod) {
				glog.V(5).Infof("[ADD] Pod %s/%s\n", pod.Namespace, pod.Name)
				updatePodInfoMap(pod)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			if util.RequestsHCU(pod) {
				glog.V(5).Infof("[UPDATE] GPU Pod %s/%s, Phase: %s\n", pod.Namespace, pod.Name, pod.Status.Phase)
				updatePodInfoMap(pod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if util.RequestsHCU(pod) {
				glog.V(5).Infof("[DELETE] Pod %s/%s\n", pod.Namespace, pod.Name)
				updatePodInfoMap(pod)
			}
		},
	})

	glog.V(2).Infof("Starting pod watcher on node %s...\n", nodeName)
	go podInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, podInformer.HasSynced)

	<-stopCh
}

// k8s指标信息更新
func updatePodInfoMap(pod *v1.Pod) {
	resourceNames := util.GetHCUResourceNames(pod)

	// 获取pod resources指标数据(hcu)
	if util.HasIntersection(resourceNames, util.StringSliceToSet(resources)) {
		podresource := podresources.NewPodResourcesClient(timeout, socket, resources, maxSize)
		podInfoMap, _ = podresource.GetDeviceToPodInfo(pod.Spec.NodeName)
		glog.V(5).Infof(" hcuPodInfoMap: %v \n", podInfoMap)
	}

	// 获取pod resources指标数据(hcu-share)
	if util.HasIntersection(resourceNames, util.StringSliceToSet(vhcuShareResource)) {
		podHCUShareResource := podresources.NewPodResourcesClient(timeout, socket, vhcuShareResource, maxSize)
		podHCUShareInfoMap, _ = podHCUShareResource.GetDeviceToPodInfo(pod.Spec.NodeName)
		glog.V(5).Infof(" podHCUShareInfoMap: %v \n", podHCUShareInfoMap)
	}

	// 获取pod resources指标数据(hcunum)
	if util.HasIntersection(resourceNames, util.StringSliceToSet(vhcuDynamicResource)) {
		podHCUHAMiResource := podresources.NewPodResourcesClient(timeout, socket, vhcuDynamicResource, maxSize)
		podHCUHAMiInfoMap, _ := podHCUHAMiResource.GetDeviceToPodInfo(pod.Spec.NodeName)
		vHCUDynamicInfoMap, hcuDynamicInfoMap, err := podresources.GetVHCUPodInfo(podHCUHAMiInfoMap)
		if err != nil {
			glog.Errorf("Get vhcu pod info error: %v ", err)
		}
		podHCUDynamicInfoMap = vHCUDynamicInfoMap
		podInfoMap = hcuDynamicInfoMap
		glog.V(5).Infof(" podHCUDynamicInfoMap: %v \n", podHCUDynamicInfoMap)
		glog.V(5).Infof(" dynamic hcuPodInfoMap: %v \n", podInfoMap)
	}
}
