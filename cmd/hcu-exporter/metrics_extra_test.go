// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

func TestParseMemBankNumericFields(t *testing.T) {
	got := parseMemBankNumericFields("ce=12\nue: 3\nignored text\nsize 1024\n")
	if got["ce"] != 12 {
		t.Fatalf("ce=%v, want 12", got["ce"])
	}
	if got["ue"] != 3 {
		t.Fatalf("ue=%v, want 3", got["ue"])
	}
	if got["size"] != 1024 {
		t.Fatalf("size=%v, want 1024", got["size"])
	}
}

func TestBoolToFloat(t *testing.T) {
	if boolToFloat(true) != 1 || boolToFloat(false) != 0 {
		t.Fatal("boolToFloat mismatch")
	}
}

func TestHealthStatusValue(t *testing.T) {
	if healthStatusValue("Healthy") != 1 || healthStatusValue("healthy") != 1 {
		t.Fatal("Healthy should be 1")
	}
	if healthStatusValue("Unhealthy") != 0 || healthStatusValue("Unknown") != 0 {
		t.Fatal("non-Healthy should be 0")
	}
}

func TestNormalizeBusID(t *testing.T) {
	if normalizeBusID(" 0000:09:00.0 ") != "0000:09:00.0" {
		t.Fatal("normalizeBusID trim/lower failed")
	}
}
