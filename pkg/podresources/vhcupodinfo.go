// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package podresources

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/HYGON-AI/hcu-dcgm/v3/pkg/dcgm"
)

const (
	VIRTUAL_HCU_CONF_DIR = "/etc/vdev"
)

type VitualDeviceInfo struct {
	//虚拟设备ID
	VitualDeviceIndex int

	//虚拟设备对应物理设备ID
	DeviceIndex int

	//虚拟设备对应物理设备号
	DeviceID string

	//设备名称
	SubSystemName string

	//温度
	Temperature float64

	//时钟频率
	Clk float64

	//虚拟设备计算单元分配量
	ComputeUnitCount int

	//虚拟设备内存分配量
	MemoryCap int

	// MemoryUsed 虚拟设备已使用的内存
	MemoryUsed int

	// UtilizationRate 虚拟设备的利用率
	UtilizationRate int
}

// parseDynamicMarkFileName parses device-plugin mark files named
// "{podUID}_{containerName}_{deviceIdx}_{vDeviceIdx}".
// Pod UIDs use hyphens (no underscores); container names may contain underscores,
// so device indices are taken from the rightmost two underscore-separated fields.
func parseDynamicMarkFileName(name string) (uid, container string, didx, vdidx int, ok bool) {
	parts := strings.Split(name, "_")
	if len(parts) < 4 {
		return "", "", 0, 0, false
	}
	vdidx, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return "", "", 0, 0, false
	}
	didx, err = strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return "", "", 0, 0, false
	}
	uid = parts[0]
	container = strings.Join(parts[1:len(parts)-2], "_")
	if uid == "" || container == "" {
		return "", "", 0, 0, false
	}
	return uid, container, didx, vdidx, true
}

// podInfoForMark resolves Pod/Namespace from any Pod Resources entry with the
// same Pod UID, then sets Container from the mark file.
//
// Init and app containers on the same physical HCU often share one device UUID
// in the Pod Resources API, so the UUID→PodInfo map keeps only the last
// container. Matching by UID + mark-file container name restores per-vHCU labels.
func podInfoForMark(uid, container string, podHCUHAMiInfoMap map[string]PodInfo) (PodInfo, bool) {
	for _, val := range podHCUHAMiInfoMap {
		if val.UID != uid {
			continue
		}
		info := val
		info.Container = container
		return info, true
	}
	return PodInfo{}, false
}

func GetVHCUPodInfo(podHCUHAMiInfoMap map[string]PodInfo) (map[string]PodInfo, map[string]PodInfo, error) {
	return getVHCUPodInfoFromDir(VIRTUAL_HCU_CONF_DIR+"/dynamic", podHCUHAMiInfoMap)
}

func getVHCUPodInfoFromDir(dir string, podHCUHAMiInfoMap map[string]PodInfo) (map[string]PodInfo, map[string]PodInfo, error) {
	hcuInfoMap := make(map[string]PodInfo)
	vhcuInfoMap := make(map[string]PodInfo)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		uid, container, didx, vdidx, ok := parseDynamicMarkFileName(info.Name())
		if !ok {
			return nil
		}
		podInfo, found := podInfoForMark(uid, container, podHCUHAMiInfoMap)
		if !found {
			return nil
		}

		if vdidx > -1 {
			vhcuInfoMap[strconv.Itoa(vdidx)] = podInfo
		} else {
			bus, busErr := dcgm.GetBus(didx)
			if busErr != nil {
				return nil
			}
			hcuInfoMap[bus] = podInfo
		}
		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("error walking directory %s: %w", dir, err)
	}

	return vhcuInfoMap, hcuInfoMap, nil
}
