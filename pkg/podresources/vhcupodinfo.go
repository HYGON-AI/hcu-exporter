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
	VIRTUAL_HCU_CONF_DIR          = "/etc/vdev"
	SHARE_HCU_RESOURCE_DIR_PREFIX = "/var/lib/kubelet/device-plugins/hygon.com_hcu-share-"
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

func GetVHCUPodInfo(podHCUHAMiInfoMap map[string]PodInfo) (map[string]PodInfo, map[string]PodInfo, error) {
	hcuInfoMap := make(map[string]PodInfo)
	vhcuInfoMap := make(map[string]PodInfo)

	err := filepath.Walk(VIRTUAL_HCU_CONF_DIR+"/dynamic", func(path string, info os.FileInfo, err error) error {
		found := false
		podInfo := PodInfo{}

		if err != nil {
			return err
		}

		if strings.Contains(info.Name(), "_") {
			for _, val := range podHCUHAMiInfoMap {
				tmpstr := strings.Split(info.Name(), "_")
				containerName := tmpstr[0] + "_" + tmpstr[1]
				if containerName == val.UID+"_"+val.Container {
					found = true
					podInfo = val
					break
				}
			}
		}

		if found {
			var didx, vdidx int
			tmpstr := strings.Split(info.Name(), "_")
			didx, _ = strconv.Atoi(tmpstr[2])
			vdidx, _ = strconv.Atoi(tmpstr[3])
			if vdidx > -1 {
				vhcuInfoMap[tmpstr[3]] = podInfo
			} else {
				bus, _ := dcgm.GetBus(didx)
				hcuInfoMap[bus] = podInfo
			}
		}

		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("error walking directory %s: %w", VIRTUAL_HCU_CONF_DIR+"/dynamic", err)
	}

	return vhcuInfoMap, hcuInfoMap, nil
}
