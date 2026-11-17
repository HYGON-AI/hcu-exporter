// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/HYGON-AI/hcu-dcgm/v3/pkg/dcgm"
	"github.com/golang/glog"
)

const dcgmDeviceReconcileInterval = 30 * time.Second

var dcgmReconcileMu sync.Mutex

// CountChengduDevicesByLspci counts Chengdu Display/Co-processor devices via lspci.
func CountChengduDevicesByLspci() (int, error) {
	cmd := exec.Command("sh", "-c", "lspci | grep Display | grep Chengdu || lspci | grep Co-processor | grep Chengdu")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return 0, nil
		}
		return 0, fmt.Errorf("lspci query failed: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0, nil
	}
	return len(strings.Split(trimmed, "\n")), nil
}

// ReconcileDCGMDevices compares RSMI, DMI (DeviceCount), and lspci device counts;
// if any side mismatches, shuts down and re-initializes DCGM via Init().
func ReconcileDCGMDevices() {
	dcgmReconcileMu.Lock()
	defer dcgmReconcileMu.Unlock()

	rsmiCount, err := dcgm.NumMonitorDevices()
	if err != nil {
		glog.Errorf("NumMonitorDevices failed: %v", err)
		return
	}

	dmiCount, err := dcgm.DeviceCount()
	if err != nil {
		glog.Errorf("DeviceCount failed: %v", err)
		if err := dcgm.ShutDown(); err != nil {
			glog.Errorf("DCGM ShutDown failed: %v", err)
		}
		if err := dcgm.Init(); err != nil {
			glog.Errorf("DCGM Init failed: %v", err)
			return
		}
		return
	}

	lspciCount, err := CountChengduDevicesByLspci()
	if err != nil {
		glog.Errorf("CountChengduDevicesByLspci failed: %v", err)
		return
	}

	glog.V(2).Infof("DCGM get hcu count ::: rsmi=%d  dmi=%d  lspci=%d", rsmiCount, dmiCount, lspciCount)
	countSynced := rsmiCount == dmiCount && dmiCount == lspciCount
	incomplete := hasIncompletePhysicalDevices()
	if countSynced && !incomplete {
		return
	}

	if !countSynced {
		reinitDCGM(fmt.Sprintf("out of sync rsmi=%d dmi=%d lspci=%d", rsmiCount, dmiCount, lspciCount))
		return
	}
	reinitDCGM(fmt.Sprintf("incomplete device info with synced count=%d", dmiCount))

}

// StartDCGMDeviceReconcileLoop starts a background loop that reconciles DCGM devices periodically.
func StartDCGMDeviceReconcileLoop() {
	go func() {
		ticker := time.NewTicker(dcgmDeviceReconcileInterval)
		defer ticker.Stop()
		for range ticker.C {
			ReconcileDCGMDevices()
		}
	}()
	glog.V(2).Infof("Started DCGM device reconcile loop, interval: %v", dcgmDeviceReconcileInterval)
}

func reinitDCGM(reason string) {
	glog.Warningf("DCGM reinitializing: %s", reason)
	if err := dcgm.ShutDown(); err != nil {
		glog.Errorf("DCGM ShutDown failed: %v", err)
	}
	if err := dcgm.Init(); err != nil {
		glog.Errorf("DCGM Init failed: %v", err)
		return
	}
	glog.V(2).Infof("DCGM reinitialized successfully")
}

// hasIncompletePhysicalDevices reports devices visible via RSMI but missing DMI fields.
// After HCU reset, RSMI/DMI/lspci counts can already match while DeviceInfos still
// returns cu=0 / empty type; that stale view must trigger DCGM reinitialization.
func hasIncompletePhysicalDevices() bool {
	devices, err := dcgm.DeviceInfos()
	if err != nil {
		glog.Errorf("DeviceInfos failed while checking completeness: %v", err)
		return true
	}
	for _, d := range devices {
		if d.MemoryTotal <= 0 || d.ComputeUnit <= 0 || d.DevTypeName == "" {
			glog.V(2).Infof("Incomplete HCU detected: id=%s cu=%v type=%q", d.DeviceId, d.ComputeUnit, d.DevTypeName)
			return true
		}
	}
	return false
}
