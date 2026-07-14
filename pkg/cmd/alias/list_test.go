package alias

import (
	"bytes"
	"strings"
	"testing"
)

// TestListCmdRegistersOutputFlag verifies the -o flag is present with the
// expected shorthand and defaults. Full end-to-end formatting is exercised
// through manual runs against a real config; this test protects the wiring
// so it doesn't silently regress.
func TestListCmdRegistersOutputFlag(t *testing.T) {
	cmd := NewCmdAliasList()

	flag := cmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("expected --output flag on alias list")
	}
	if flag.Shorthand != "o" {
		t.Errorf("expected -o shorthand, got %q", flag.Shorthand)
	}
	if flag.DefValue != "" {
		t.Errorf("expected empty default, got %q", flag.DefValue)
	}

	// Help text should mention yaml and json so users can discover them.
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	_ = cmd.Help()
	help := buf.String()
	for _, want := range []string{"yaml", "json", "table"} {
		if !strings.Contains(help, want) {
			t.Errorf("expected help text to mention %q; got:\n%s", want, help)
		}
	}
}
