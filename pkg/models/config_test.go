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

func TestResolveDefaultTemplate_Precedence(t *testing.T) {
	c := &Config{
		DefaultTemplateContext:   map[string]string{"prod": "compact", "dev": "verbose"},
		DefaultTemplateNamespace: map[string]string{"kube-system": "minimal", "monitoring": "full"},
	}
	cases := []struct {
		name    string
		kubeCtx string
		ns      string
		want    string
	}{
		{"namespace beats context", "prod", "kube-system", "minimal"},
		{"context used when namespace has no override", "prod", "default", "compact"},
		{"context-only match", "dev", "", "verbose"},
		{"namespace-only match", "", "monitoring", "full"},
		{"no match falls back to default", "staging", "team-x", "default"},
		{"empty inputs fall back to default", "", "", "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.ResolveDefaultTemplate(tc.kubeCtx, tc.ns); got != tc.want {
				t.Errorf("ResolveDefaultTemplate(%q, %q) = %q, want %q", tc.kubeCtx, tc.ns, got, tc.want)
			}
		})
	}
}

func TestResolveDefaultTemplate_EmptyStringOverrideIgnored(t *testing.T) {
	// A map entry present but set to "" must not shadow the fallback — the
	// implementation guards with `t != ""`.
	c := &Config{
		DefaultTemplateNamespace: map[string]string{"ns": ""},
		DefaultTemplateContext:   map[string]string{"ctx": ""},
	}
	if got := c.ResolveDefaultTemplate("ctx", "ns"); got != "default" {
		t.Errorf("empty-string overrides should be ignored; got %q", got)
	}
}

func TestResolveDefaultTemplate_NilMaps(t *testing.T) {
	c := &Config{}
	if got := c.ResolveDefaultTemplate("prod", "kube-system"); got != "default" {
		t.Errorf("nil maps should yield default; got %q", got)
	}
}
