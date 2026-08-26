// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/HYGON-AI/hcu-exporter/v3/pkg/podresources"
	"github.com/HYGON-AI/hcu-exporter/v3/pkg/util"

	"github.com/HYGON-AI/hcu-dcgm/v3/pkg/dcgm"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
)

// 采集数据并设置collector值
func recordMetrics() {
	go runMetricsCollectLoop()
	go watchMetricsStall()
}

func runMetricsCollectLoop() {
	nodeName := resolveNodeName()
	if nodeName == "" {
		return
	}
	glog.V(2).Infof("Get NodeName : %s \n", nodeName)
	maybeStartPodInformer(nodeName)

	for {
		vdeviceInfos, err := dcgm.VDeviceInfos()
		if err != nil {
			glog.Errorf("Get vdevice error: %v ", err)
			time.Sleep(10 * time.Second)
			continue
		}
		glog.V(3).Infof("Get vdevices number : %d \n", len(vdeviceInfos))

		deviceInfos, err := dcgm.DeviceInfos()
		if err != nil {
			glog.Errorf("Get device error: %v ", err)
			time.Sleep(10 * time.Second)
			continue
		}
		glog.V(3).Infof("Get devices number : %d \n", len(deviceInfos))

		vhcuInfoMap := buildVHCUInfoMap(vdeviceInfos)
		metricsHCUCollectMap := collectAllHCUMetrics(deviceInfos)
		metricsVHCUCollectMap := collectAllVHCUMetrics(vdeviceInfos)

		collectorReset()
		vhcuCollectorReset()
		exportHCUMetrics(deviceInfos, metricsHCUCollectMap, nodeName)
		exportVHCUMetrics(vhcuInfoMap, metricsVHCUCollectMap, nodeName)

		os.Setenv("COLLECT_TIME", fmt.Sprintf("%d", time.Now().Unix()))
		time.Sleep(time.Duration(pulse) * time.Second)
	}
}

func resolveNodeName() string {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName != "" {
		return nodeName
	}
	cmd := exec.Command("cat", "/etc/hostname")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		glog.Errorf(fmt.Sprint(err) + ": " + stderr.String())
		return ""
	}
	return strings.TrimSpace(out.String())
}

func maybeStartPodInformer(nodeName string) {
	if connectK8sFlag && util.FileExists(socket) {
		glog.V(2).Infof("K8S exists~")
		config, err := podresources.BuildConfig(podresources.Kubeconfig)
		if err != nil {
			_ = fmt.Errorf("%v", err)
		}
		client, err := kubernetes.NewForConfig(config)
		_ = err
		go informerPodHandler(nodeName, client)
		return
	}
	if !connectK8sFlag {
		glog.V(2).Infof("K8S connection disabled by connect-k8s=false")
	}
}

func buildVHCUInfoMap(vdeviceInfos []dcgm.VDeviceInfo) map[string]dcgm.VDeviceInfo {
	vhcuInfoMap := make(map[string]dcgm.VDeviceInfo)
	vhcuShareResource = []string{"hygon.com/hcu-share"}
	for _, vdeviceInfo := range vdeviceInfos {
		vhcuInfoMap[strconv.Itoa(vdeviceInfo.VdvInd)] = vdeviceInfo
		cuString := strconv.Itoa(vdeviceInfo.VComputeUnitCount) + "c"
		memString := strconv.Itoa(int(math.Round(float64(vdeviceInfo.VMemoryTotal)/(1024*1024*1024)))) + "g"
		prefixList := util.GetResourceNamePrefixList()
		if connectK8sFlag && util.FileExists(socket) {
			for _, prefix := range prefixList {
				vhcuShareResource = append(vhcuShareResource, "hygon.com/"+prefix+"-share-"+cuString+"-"+memString)
			}
		}
	}
	glog.V(5).Infof("Get vhcu info : %v \n", vhcuInfoMap)
	return vhcuInfoMap
}

func collectAllHCUMetrics(deviceInfos []dcgm.DeviceInfo) map[int]map[string]float64 {
	metricsHCUCollectMap := make(map[int]map[string]float64)
	for _, info := range deviceInfos {
		if _, exists := metricsHCUCollectMap[info.DvInd]; !exists {
			metricsHCUCollectMap[info.DvInd] = make(map[string]float64)
		}
		collectDeviceMetricValues(info, metricsHCUCollectMap[info.DvInd])
		collectExtraDeviceMetrics(info.DvInd, deviceInfos, metricsHCUCollectMap[info.DvInd])
	}
	collectHealthMetrics(deviceInfos, metricsHCUCollectMap)
	collectProcessMetrics(deviceInfos, metricsHCUCollectMap)
	return metricsHCUCollectMap
}

func collectDeviceMetricValues(info dcgm.DeviceInfo, out map[string]float64) {
	pcieBandwidth, pcieBwFetched := dcgm.PcieBandwidthInfo{}, false
	for _, metrics := range metricsList {
		if collectECCMetric(info.DvInd, metrics, out) {
			continue
		}
		if collectHylinkMetric(info.DvInd, metrics, out) {
			continue
		}
		if strings.EqualFold(metrics, "hcu_se_usage") {
			seUsage, err := dcgm.HCUSEUsage(info.DvInd)
			if err != nil {
				glog.Errorf("Get SE Usage Error: %v", err)
			}
			for seID, percent := range seUsage.Percent {
				out[metrics+"-"+strconv.Itoa(seID)] = float64(percent)
			}
			continue
		}
		if strings.EqualFold(metrics, "hcu_pciebw_mb") ||
			strings.EqualFold(metrics, "hcu_pcie_receive_mb") ||
			strings.EqualFold(metrics, "hcu_pcie_sent_mb") {
			if !pcieBwFetched {
				var err error
				pcieBandwidth, err = dcgm.PcieBw(info.DvInd)
				if err != nil {
					glog.Errorf("Get Pciebw error: %v", err)
				}
				pcieBwFetched = true
			}
			switch metrics {
			case "hcu_pciebw_mb":
				out[metrics] = float64(pcieBandwidth.Sent + pcieBandwidth.Received)
			case "hcu_pcie_receive_mb":
				out[metrics] = float64(pcieBandwidth.Received)
			case "hcu_pcie_sent_mb":
				out[metrics] = float64(pcieBandwidth.Sent)
			}
			continue
		}
		if metricsFunc, exist := hcuFunctionMap[metrics]; exist {
			out[metrics] = metricsFunc(info.DvInd)
		}
	}
}

func collectECCMetric(dvInd int, metrics string, out map[string]float64) bool {
	if !strings.Contains(metrics, "e_count") {
		return false
	}
	blockInfos, err := dcgm.EccBlocksInfo(dvInd)
	if err != nil {
		glog.Errorf("Get BlockInfo Error: %v", err)
	}
	switch {
	case strings.EqualFold(metrics, "hcu_ce_count"):
		for _, blockInfo := range blockInfos {
			out[metrics+"-"+blockInfo.Block] = float64(blockInfo.CE)
		}
		return true
	case strings.EqualFold(metrics, "hcu_ue_count"):
		for _, blockInfo := range blockInfos {
			out[metrics+"-"+blockInfo.Block] = float64(blockInfo.UE)
		}
		return true
	case strings.EqualFold(metrics, "hcu_ce_count_total"):
		var total float64
		for _, blockInfo := range blockInfos {
			total += float64(blockInfo.CE)
		}
		out[metrics] = total
		return true
	case strings.EqualFold(metrics, "hcu_ue_count_total"):
		var total float64
		for _, blockInfo := range blockInfos {
			total += float64(blockInfo.UE)
		}
		out[metrics] = total
		return true
	}
	return false
}

func collectHylinkMetric(dvInd int, metrics string, out map[string]float64) bool {
	if !strings.Contains(metrics, "hylink") {
		return false
	}
	linkStatus, err := dcgm.HyLinkStatusByHcuId(dvInd)
	if err != nil {
		glog.Errorf("Get Hylink Status Error: %v", err)
	}
	switch {
	case strings.EqualFold(metrics, "hcu_hylink_send"):
		out[metrics] = linkStatus.Send
		if hylinkDetailFlag {
			for _, hylinkDetail := range linkStatus.Links {
				out[metrics+"-"+strconv.Itoa(hylinkDetail.LinkId)] = hylinkDetail.Send
			}
		}
		return true
	case strings.EqualFold(metrics, "hcu_hylink_recv"):
		out[metrics] = linkStatus.Recv
		if hylinkDetailFlag {
			for _, hylinkDetail := range linkStatus.Links {
				out[metrics+"-"+strconv.Itoa(hylinkDetail.LinkId)] = hylinkDetail.Recv
			}
		}
		return true
	case strings.EqualFold(metrics, "hcu_hylink_send_recv"):
		out[metrics] = linkStatus.Recv
		if hylinkDetailFlag {
			for _, hylinkDetail := range linkStatus.Links {
				out[metrics+"-"+strconv.Itoa(hylinkDetail.LinkId)] = hylinkDetail.Recv + hylinkDetail.Send
			}
		}
		return true
	}
	return false
}

func collectAllVHCUMetrics(vdeviceInfos []dcgm.VDeviceInfo) map[int]map[string]float64 {
	metricsVHCUCollectMap := make(map[int]map[string]float64)
	for _, info := range vdeviceInfos {
		if _, exists := metricsVHCUCollectMap[info.VdvInd]; !exists {
			metricsVHCUCollectMap[info.VdvInd] = make(map[string]float64)
		}
		for _, metrics := range metricsList {
			if metricsFunc, exist := vhcuFunctionMap[metrics]; exist {
				metricsVHCUCollectMap[info.VdvInd][metrics] = metricsFunc(info)
			}
		}
	}
	return metricsVHCUCollectMap
}

func exportHCUMetrics(deviceInfos []dcgm.DeviceInfo, metricsHCUCollectMap map[int]map[string]float64, nodeName string) {
	for _, info := range deviceInfos {
		labels := buildHCULabels(info, nodeName)
		for metrics, metricsValue := range metricsHCUCollectMap[info.DvInd] {
			clearEphemeralLabels(labels)
			if exportCompoundHCUMetric(metrics, metricsValue, labels) {
				continue
			}
			if setExtraMetricValue(metrics, labels, metricsValue) {
				continue
			}
			setMetricValue(metrics, labels, metricsValue)
		}
		glog.V(5).Infof("hcu info : %v \n", info)
	}
}

func buildHCULabels(info dcgm.DeviceInfo, nodeName string) prometheus.Labels {
	podInfo, exists := podInfoMap[info.PciBusNumber]
	labels := prometheus.Labels{
		"device_id":         info.DeviceId,
		"minor_number":      strconv.Itoa(info.DvInd),
		"name":              info.DevTypeName,
		"node":              nodeName,
		"pcieBus_number":    info.PciBusNumber,
		"hcu_pod_namespace": "",
		"hcu_pod_name":      "",
		"container":         "",
	}
	if exists {
		labels["hcu_pod_namespace"] = podInfo.Namespace
		labels["hcu_pod_name"] = podInfo.Pod
		labels["container"] = podInfo.Container
	}
	return labels
}

func exportCompoundHCUMetric(metrics string, metricsValue float64, labels prometheus.Labels) bool {
	if strings.HasPrefix(metrics, "hcu_ce_count-") || strings.HasPrefix(metrics, "hcu_ue_count-") {
		parts := strings.SplitN(metrics, "-", 2)
		labels["block_type"] = parts[1]
		setMetricValue(parts[0], labels, metricsValue)
		return true
	}
	if strings.Contains(metrics, "hylink") {
		if strings.Contains(metrics, "-") {
			labels["link_id"] = strings.Split(metrics, "-")[1]
			setMetricValue(strings.Split(metrics, "-")[0], labels, metricsValue)
		} else {
			labels["link_id"] = "all"
			setMetricValue(strings.Split(metrics, "-")[0], labels, metricsValue)
		}
		return true
	}
	if strings.HasPrefix(metrics, "hcu_se_usage-") {
		labels["se_id"] = strings.TrimPrefix(metrics, "hcu_se_usage-")
		setMetricValue("hcu_se_usage", labels, metricsValue)
		return true
	}
	return false
}

func exportVHCUMetrics(vhcuInfoMap map[string]dcgm.VDeviceInfo, metricsVHCUCollectMap map[int]map[string]float64, nodeName string) {
	for _, info := range vhcuInfoMap {
		labels := buildVHCULabels(info, nodeName)
		for metrics, metricsValue := range metricsVHCUCollectMap[info.VdvInd] {
			setMetricValue(metrics, labels, metricsValue)
		}
		glog.V(5).Infof("vhcu info : %v \n", info)
	}
}

func buildVHCULabels(info dcgm.VDeviceInfo, nodeName string) prometheus.Labels {
	podShareInfo, existsShare := podHCUShareInfoMap["vdev"+strconv.Itoa(info.VdvInd)]
	podDynamicInfo, existsDynamic := podHCUDynamicInfoMap[strconv.Itoa(info.VdvInd)]
	deviceId, err := dcgm.GetDeviceId(info.DvInd)
	if err != nil {
		glog.Errorf("Get deviceId error %v", err)
	}

	labels := prometheus.Labels{
		"device_id":          deviceId,
		"minor_number":       strconv.Itoa(info.DvInd),
		"name":               info.Name,
		"node":               nodeName,
		"hcu_pod_namespace":  "",
		"hcu_pod_name":       "",
		"container":          "",
		"vhcu_minor_number":  strconv.Itoa(info.VdvInd),
		"vhcu_computer_unit": strconv.Itoa(info.VComputeUnitCount),
		"vhcu_memory_cap":    strconv.Itoa(int(info.VMemoryTotal)),
	}
	switch {
	case existsShare:
		labels["hcu_pod_namespace"] = podShareInfo.Namespace
		labels["hcu_pod_name"] = podShareInfo.Pod
		labels["container"] = podShareInfo.Container
	case existsDynamic:
		labels["hcu_pod_namespace"] = podDynamicInfo.Namespace
		labels["hcu_pod_name"] = podDynamicInfo.Pod
		labels["container"] = podDynamicInfo.Container
	}
	return labels
}

func watchMetricsStall() {
	for {
		collectTimeEnv := os.Getenv("COLLECT_TIME")
		if collectTimeEnv == "" {
			glog.V(3).Infoln("Collect Time Env not Set")
			time.Sleep(time.Duration(pulse*6) * time.Second)
			collectTimeEnv = os.Getenv("COLLECT_TIME")
		}

		collectTime, err := strconv.ParseInt(collectTimeEnv, 10, 64)
		if err != nil {
			glog.Errorf("COLLECT_TIME Conv Error: %v", err)
		}

		nowTime := time.Now().Unix()
		diff := nowTime - collectTime
		if diff > 60 {
			glog.Error("Metrics Collect Stuck, Please Restart HCU-Exporter Service!")
			os.Exit(1)
		}
		time.Sleep(time.Duration(pulse) * time.Second)
	}
}
