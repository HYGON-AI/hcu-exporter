// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"strings"
)

const (
	// ResourceNamespace is the vendor prefix for all K8s resource names
	// (hygon.com/<family>…), matching k8s-hcu-device-plugin.
	ResourceNamespace = "hygon.com"

	// DefaultResourcePrefix is the product-family prefix used in K8s resource
	// names (hcu → hygon.com/hcu, hygon.com/hcunum, …).
	DefaultResourcePrefix = "hcu"
)

// resourcePrefix is the lowercase product-family prefix (hcu|*cu|…).
// Mutated only by InitResourcePrefix at process start.
var resourcePrefix = DefaultResourcePrefix

// InitResourcePrefix validates and applies RESOURCE_PREFIX.
// Must be called once at startup before building Pod Resources name lists.
// Keep this value identical to k8s-hcu-device-plugin's RESOURCE_PREFIX.
func InitResourcePrefix(prefix string) error {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		prefix = DefaultResourcePrefix
	}
	// Same rule as device-plugin: family token must contain "cu"
	// (hcu, *cu, …).
	if !strings.Contains(prefix, "cu") {
		return fmt.Errorf("invalid resource-prefix %q, want a product family containing \"cu\" (e.g. hcu, *cu)", prefix)
	}
	resourcePrefix = prefix
	return nil
}

// ResourcePrefix returns the lowercase product-family prefix (hcu|*cu|…).
func ResourcePrefix() string {
	return resourcePrefix
}

// ResourceNameNum returns hcunum / *cunum .
func ResourceNameNum() string {
	return resourcePrefix + "num"
}

// ResourceNameCores returns hcucores / *cucores .
func ResourceNameCores() string {
	return resourcePrefix + "cores"
}

// ResourceNameMem returns hcumem / *cumem .
func ResourceNameMem() string {
	return resourcePrefix + "mem"
}

// WholeCardResourceName returns hygon.com/hcu (or …/*cu).
func WholeCardResourceName() string {
	return ResourceNamespace + "/" + ResourcePrefix()
}

// DynamicNumResourceName returns hygon.com/hcunum (or …/*cunum).
func DynamicNumResourceName() string {
	return ResourceNamespace + "/" + ResourceNameNum()
}

// ShareResourceBaseName returns hygon.com/hcu-share (or …/*cu-share, …).
func ShareResourceBaseName() string {
	return ResourceNamespace + "/" + ResourcePrefix() + "-share"
}
