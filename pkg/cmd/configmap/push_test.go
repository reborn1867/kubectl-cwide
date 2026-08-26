package configmap

import "testing"

func TestDeriveKeyForFile_WithOverride(t *testing.T) {
	got, err := deriveKeyForFile("/tmp/whatever.yaml", "pod--v1..debug")
	if err != nil {
		t.Fatalf("override rejected: %v", err)
	}
	if got != "pod--v1..debug" {
		t.Errorf("override not returned verbatim: %q", got)
	}
}

func TestDeriveKeyForFile_OverrideMustContainDotDot(t *testing.T) {
	_, err := deriveKeyForFile("/tmp/whatever.yaml", "pod-v1-debug")
	if err == nil {
		t.Fatal("expected error for malformed --key")
	}
}

func TestDeriveKeyForFile_DerivesFromParentDir(t *testing.T) {
	got, err := deriveKeyForFile("pod--v1/debug.yaml", "")
	if err != nil {
		t.Fatalf("derivation failed: %v", err)
	}
	if got != "pod--v1..debug" {
		t.Errorf("derived key = %q, want pod--v1..debug", got)
	}
}

func TestDeriveKeyForFile_ParentDirWithoutDashIsRejected(t *testing.T) {
	// "downloads" isn't a cwide resource dir shape → must error asking for --key
	_, err := deriveKeyForFile("/home/me/downloads/debug.yaml", "")
	if err == nil {
		t.Fatal("expected error when parent dir isn't a resource dir")
	}
}

func TestDeriveKeyForFile_NoExtensionRejected(t *testing.T) {
	_, err := deriveKeyForFile("pod--v1/debug", "")
	if err == nil {
		t.Fatal("expected error when file has no extension")
	}
}
