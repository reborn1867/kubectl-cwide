package models

import "testing"

func TestResolveAliasTarget_RichWinsOverLegacy(t *testing.T) {
	c := &Config{
		Aliases:      map[string]string{"pd": "podLegacy"},
		AliasEntries: map[string]AliasEntry{"pd": {Resource: "pods"}},
	}
	if got := c.ResolveAliasTarget("pd"); got != "pods" {
		t.Errorf("expected rich entry to win, got %q", got)
	}
}

func TestResolveAliasTarget_LegacyFallback(t *testing.T) {
	c := &Config{Aliases: map[string]string{"pd": "pods"}}
	if got := c.ResolveAliasTarget("pd"); got != "pods" {
		t.Errorf("legacy lookup: got %q", got)
	}
	if got := c.ResolveAliasTarget("missing"); got != "" {
		t.Errorf("missing alias should return empty, got %q", got)
	}
}

func TestResolveAliasTemplate_PerKindWinsOverGeneral(t *testing.T) {
	c := &Config{AliasEntries: map[string]AliasEntry{
		"core": {
			Resource:  "pod,svc",
			Template:  "wide",
			Templates: map[string]string{"pod": "debug"},
		},
	}}
	if got := c.ResolveAliasTemplate("core", "pod"); got != "debug" {
		t.Errorf("expected per-kind pod=debug, got %q", got)
	}
	if got := c.ResolveAliasTemplate("core", "svc"); got != "wide" {
		t.Errorf("expected general template for svc, got %q", got)
	}
	if got := c.ResolveAliasTemplate("core", ""); got != "wide" {
		t.Errorf("empty kind should hit general template, got %q", got)
	}
}

func TestResolveAliasTemplate_UnboundAliasReturnsEmpty(t *testing.T) {
	c := &Config{Aliases: map[string]string{"pd": "pods"}}
	if got := c.ResolveAliasTemplate("pd", "pod"); got != "" {
		t.Errorf("legacy alias has no template binding; got %q", got)
	}
	if got := c.ResolveAliasTemplate("missing", ""); got != "" {
		t.Errorf("missing alias should have no template; got %q", got)
	}
}
