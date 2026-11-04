// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package util

import "testing"

func TestInitResourcePrefix(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})

	if err := InitResourcePrefix(""); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if ResourcePrefix() != DefaultResourcePrefix {
		t.Fatalf("empty should default to %q, got %q", DefaultResourcePrefix, ResourcePrefix())
	}

	if err := InitResourcePrefix("*cu"); err != nil {
		t.Fatalf("*cu: %v", err)
	}
	if ResourcePrefix() != "*cu" || ResourceNameNum() != "*cunum" || ResourceNameMem() != "*cumem" || ResourceNameCores() != "*cucores" {
		t.Fatalf("*cu names: prefix=%q num=%q mem=%q cores=%q",
			ResourcePrefix(), ResourceNameNum(), ResourceNameMem(), ResourceNameCores())
	}
	if DynamicNumResourceName() != "hygon.com/*cunum" || WholeCardResourceName() != "hygon.com/*cu" || ShareResourceBaseName() != "hygon.com/*cu-share" {
		t.Fatalf("*cu full names: %q %q %q", DynamicNumResourceName(), WholeCardResourceName(), ShareResourceBaseName())
	}

	if err := InitResourcePrefix("*cu"); err != nil {
		t.Fatalf("*cu: %v", err)
	}
	if ResourceNameNum() != "*cunum" || DynamicNumResourceName() != "hygon.com/*cunum" || ShareResourceBaseName() != "hygon.com/*cu-share" {
		t.Fatalf("*cu names: num=%q dyn=%q share=%q", ResourceNameNum(), DynamicNumResourceName(), ShareResourceBaseName())
	}

	if err := InitResourcePrefix("HCU"); err != nil {
		t.Fatalf("HCU: %v", err)
	}
	if ResourcePrefix() != "hcu" {
		t.Fatalf("case fold: got %q", ResourcePrefix())
	}

	if err := InitResourcePrefix("gpu"); err == nil {
		t.Fatal("expected error for gpu")
	}
}

func TestGetResourceNameListRespectsPrefix(t *testing.T) {
	t.Cleanup(func() {
		_ = InitResourcePrefix(DefaultResourcePrefix)
	})

	_ = InitResourcePrefix("*cu")
	whole := GetResourceNameList(false)
	if len(whole) == 0 || whole[0] != "hygon.com/*cu" {
		t.Fatalf("whole card list: %v", whole)
	}
	dyn := GetResourceNameList(true)
	if len(dyn) == 0 || dyn[0] != "hygon.com/*cunum" {
		t.Fatalf("dynamic list: %v", dyn)
	}
	prefixes := GetResourceNamePrefixList()
	if len(prefixes) == 0 || prefixes[0] != "*cu" {
		t.Fatalf("prefix list: %v", prefixes)
	}
}
