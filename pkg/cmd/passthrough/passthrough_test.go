package passthrough

import (
	"reflect"
	"testing"
)

func TestResolveArgsOnlyRewritesFirstNonFlag(t *testing.T) {
	// resolveArgs walks args and rewrites only the first non-flag token.
	// utils.ResolveAliasString is a no-op when no config exists (test sandbox),
	// so we're really asserting the "leave later tokens alone" behavior here.
	got := resolveArgs([]string{"pd", "my-pod", "key=value"})
	if !reflect.DeepEqual(got, []string{"pd", "my-pod", "key=value"}) {
		t.Errorf("expected identity when no alias resolves; got %v", got)
	}
}

func TestResolveArgsSkipsLeadingFlags(t *testing.T) {
	// If the user leads with a flag ("kubectl cwide delete --grace-period=0 pod/x"),
	// we still want to rewrite the resource-type token, not the flag.
	got := resolveArgs([]string{"--grace-period=0", "pod/my-pod"})
	if got[0] != "--grace-period=0" || got[1] != "pod/my-pod" {
		t.Errorf("unexpected rewrite: %v", got)
	}
}

func TestRewriteResourceTokenHandlesSlashForm(t *testing.T) {
	// With no config, ResolveAliasString returns input verbatim. The point of
	// this test is to guard the "keep the /name suffix" logic against
	// regression — if we ever accidentally lose it, the resulting kubectl
	// invocation will fail loudly, but this catches it earlier.
	if got := rewriteResourceToken("pod/my-pod"); got != "pod/my-pod" {
		t.Errorf("slash form lost or corrupted: %q", got)
	}
	if got := rewriteResourceToken("pod"); got != "pod" {
		t.Errorf("bare form corrupted: %q", got)
	}
}

func TestCobraUseHelpers(t *testing.T) {
	if cobraFirstToken("annotate TYPE NAME") != "annotate" {
		t.Fail()
	}
	if trimFirstToken("annotate TYPE NAME") != "TYPE NAME" {
		t.Fail()
	}
	if trimFirstToken("edit") != "" {
		t.Fail()
	}
}
