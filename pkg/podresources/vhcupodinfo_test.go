// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package podresources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDynamicMarkFileName(t *testing.T) {
	uid, ctr, didx, vdidx, ok := parseDynamicMarkFileName("abc-def_init-vhcu-check_0_1")
	if !ok || uid != "abc-def" || ctr != "init-vhcu-check" || didx != 0 || vdidx != 1 {
		t.Fatalf("basic: uid=%q ctr=%q didx=%d vdidx=%d ok=%v", uid, ctr, didx, vdidx, ok)
	}

	uid, ctr, didx, vdidx, ok = parseDynamicMarkFileName("abc-def_init_vhcu_check_0_-1")
	if !ok || uid != "abc-def" || ctr != "init_vhcu_check" || didx != 0 || vdidx != -1 {
		t.Fatalf("underscored container: uid=%q ctr=%q didx=%d vdidx=%d ok=%v", uid, ctr, didx, vdidx, ok)
	}

	if _, _, _, _, ok = parseDynamicMarkFileName("too_short"); ok {
		t.Fatal("expected parse failure for short name")
	}
}

func TestPodInfoForMarkUsesMarkContainerWhenUUIDCollapsed(t *testing.T) {
	// Pod Resources map only retains the app container for a shared device UUID.
	m := map[string]PodInfo{
		"HCU-serial": {
			Pod:       "hcu-init-vhcu-demo",
			Namespace: "default",
			Container: "hcu-matmul",
			UID:       "uid-1",
		},
	}
	got, ok := podInfoForMark("uid-1", "init-vhcu-check", m)
	if !ok {
		t.Fatal("expected match by pod UID")
	}
	if got.Container != "init-vhcu-check" || got.Pod != "hcu-init-vhcu-demo" || got.Namespace != "default" {
		t.Fatalf("got %+v", got)
	}
	if _, ok = podInfoForMark("other-uid", "init-vhcu-check", m); ok {
		t.Fatal("should not match unrelated UID")
	}
}

func TestGetVHCUPodInfoFromDirAssociatesInitAndApp(t *testing.T) {
	dir := t.TempDir()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mustWrite := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("ts"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(uid + "_init-vhcu-check_0_0")
	mustWrite(uid + "_hcu-matmul_0_1")

	// Collapsed Pod Resources map: only the app container remains for the UUID.
	podMap := map[string]PodInfo{
		"shared-uuid": {
			Pod:       "hcu-init-vhcu-demo",
			Namespace: "default",
			Container: "hcu-matmul",
			UID:       uid,
		},
	}

	vhcuMap, _, err := getVHCUPodInfoFromDir(dir, podMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(vhcuMap) != 2 {
		t.Fatalf("want 2 vhcu entries, got %d: %+v", len(vhcuMap), vhcuMap)
	}
	if vhcuMap["0"].Container != "init-vhcu-check" {
		t.Fatalf("vhcu 0: got %+v", vhcuMap["0"])
	}
	if vhcuMap["1"].Container != "hcu-matmul" {
		t.Fatalf("vhcu 1: got %+v", vhcuMap["1"])
	}
	if vhcuMap["0"].Pod != "hcu-init-vhcu-demo" || vhcuMap["1"].Namespace != "default" {
		t.Fatalf("pod labels missing: %+v", vhcuMap)
	}
}
