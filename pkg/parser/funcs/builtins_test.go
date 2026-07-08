package funcs

import (
	"os"
	"testing"
)

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{int64(0), "0 B"},
		{int64(1023), "1023 B"},
		{int64(1024), "1.0 KiB"},
		{int64(1024 * 1024), "1.0 MiB"},
		{int64(-2048), "-2.0 KiB"},
		{"4096", "4.0 KiB"},
		{"not-a-number", "not-a-number"},
	}
	for _, tc := range cases {
		got := HumanBytes(tc.in)
		if got != tc.want {
			t.Errorf("HumanBytes(%v) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate(5, "hello world"); got != "hello…" {
		t.Errorf("truncate = %q", got)
	}
	if got := Truncate(0, "abc"); got != "abc" {
		t.Errorf("truncate 0 = %q", got)
	}
	if got := Truncate(100, "short"); got != "short" {
		t.Errorf("no-op truncate = %q", got)
	}
}

func TestB64Dec(t *testing.T) {
	if got := B64Dec("aGVsbG8="); got != "hello" {
		t.Errorf("b64dec = %q", got)
	}
	// bad input returns as-is
	if got := B64Dec("not*base64"); got != "not*base64" {
		t.Errorf("b64dec bad = %q", got)
	}
}

func TestColorIfRespectsNoColorEnv(t *testing.T) {
	// Force enable so we start from a known state, then set NO_COLOR.
	SetColorDisabled(false)
	old, hadOld := os.LookupEnv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if hadOld {
			os.Setenv("NO_COLOR", old)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	got := ColorIf(true, "red", "err")
	if got != "err" {
		t.Errorf("expected no color escapes when NO_COLOR set; got %q", got)
	}
}

func TestSafeIndex(t *testing.T) {
	root := map[string]interface{}{
		"status": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "app"},
			},
		},
	}
	if got := SafeIndex(root, "status", "containers", 0, "name"); got != "app" {
		t.Errorf("safeIndex = %v", got)
	}
	// Missing intermediate returns empty string.
	if got := SafeIndex(root, "status", "missing", 0); got != "" {
		t.Errorf("safeIndex missing = %v", got)
	}
	// Out-of-range slice index returns empty string.
	if got := SafeIndex(root, "status", "containers", 99, "name"); got != "" {
		t.Errorf("safeIndex OOB = %v", got)
	}
}
